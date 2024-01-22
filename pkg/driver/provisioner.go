/*
 Copyright 2022 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package driver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	provisioncontroller "sigs.k8s.io/sig-storage-lib-external-provisioner/v6/controller"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
)

type provisionerService struct {
	juicefs juicefs.Interface
	*k8s.K8sClient
	leaderElection              bool
	leaderElectionNamespace     string
	leaderElectionLeaseDuration time.Duration
	metrics                     *provisionerMetrics
}

type provisionerMetrics struct {
	provisionErrors prometheus.Counter
}

func newProvisionerMetrics(reg prometheus.Registerer) *provisionerMetrics {
	metrics := &provisionerMetrics{}
	metrics.provisionErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "provision_errors",
		Help: "number of provision errors",
	})
	reg.MustRegister(metrics.provisionErrors)
	return metrics
}

func newProvisionerService(k8sClient *k8s.K8sClient, leaderElection bool,
	leaderElectionNamespace string, leaderElectionLeaseDuration time.Duration, reg prometheus.Registerer) (provisionerService, error) {
	jfs := juicefs.NewJfsProvider(nil, k8sClient)
	if leaderElectionNamespace == "" {
		leaderElectionNamespace = config.Namespace
	}
	metrics := newProvisionerMetrics(reg)
	return provisionerService{
		juicefs:                     jfs,
		K8sClient:                   k8sClient,
		leaderElection:              leaderElection,
		leaderElectionNamespace:     leaderElectionNamespace,
		leaderElectionLeaseDuration: leaderElectionLeaseDuration,
		metrics:                     metrics,
	}, nil
}

func (j *provisionerService) Run(ctx context.Context) {
	if j.K8sClient == nil {
		klog.Fatalf("K8sClient is nil")
	}
	serverVersion, err := j.K8sClient.Discovery().ServerVersion()
	if err != nil {
		klog.Fatalf("Error getting server version: %v", err)
	}
	pc := provisioncontroller.NewProvisionController(j.K8sClient,
		config.DriverName,
		j,
		serverVersion.GitVersion,
		provisioncontroller.LeaderElection(j.leaderElection),
		provisioncontroller.LeaseDuration(j.leaderElectionLeaseDuration),
		provisioncontroller.LeaderElectionNamespace(j.leaderElectionNamespace),
	)
	pc.Run(ctx)
}

func (j *provisionerService) Provision(ctx context.Context, options provisioncontroller.ProvisionOptions) (*corev1.PersistentVolume, provisioncontroller.ProvisioningState, error) {
	klog.V(6).Infof("Provisioner Provision: options %v", options)
	if options.PVC.Spec.Selector != nil {
		return nil, provisioncontroller.ProvisioningFinished, fmt.Errorf("claim Selector is not supported")
	}

	pvMeta := util.NewObjectMeta(*options.PVC, options.SelectedNode)

	pvName := options.PVName
	scParams := make(map[string]string)
	for k, v := range options.StorageClass.Parameters {
		if strings.HasPrefix(k, "csi.storage.k8s.io/") {
			scParams[k] = pvMeta.ResolveSecret(v, pvName)
		} else {
			scParams[k] = pvMeta.StringParser(options.StorageClass.Parameters[k])
		}
	}
	klog.V(6).Infof("Provisioner Resolved StorageClass.Parameters: %v", scParams)

	subPath := pvName
	if scParams["pathPattern"] != "" {
		subPath = scParams["pathPattern"]
	}
	// return error if set readonly in dynamic provisioner
	for _, am := range options.PVC.Spec.AccessModes {
		if am == corev1.ReadOnlyMany {
			if options.StorageClass.Parameters["pathPattern"] == "" {
				j.metrics.provisionErrors.Inc()
				return nil, provisioncontroller.ProvisioningFinished, status.Errorf(codes.InvalidArgument, "Dynamic mounting uses the sub-path named pv name as data isolation, so read-only mode cannot be used.")
			} else {
				klog.Warningf("Volume is set readonly, please make sure the subpath %s exists.", subPath)
			}
		}
	}

	mountOptions := make([]string, 0)
	for _, mo := range options.StorageClass.MountOptions {
		parsedStr := pvMeta.StringParser(mo)
		mountOptions = append(mountOptions, strings.Split(strings.TrimSpace(parsedStr), ",")...)
	}
	klog.V(6).Infof("Provisioner Resolved MountOptions: %v", mountOptions)

	secret, err := j.K8sClient.GetSecret(ctx, scParams[config.ProvisionerSecretName], scParams[config.ProvisionerSecretNamespace])
	if err != nil {
		klog.Errorf("[PVCReconciler]: Get Secret error: %v", err)
		j.metrics.provisionErrors.Inc()
		return nil, provisioncontroller.ProvisioningFinished, errors.New("unable to provision new pv: " + err.Error())
	}
	// set volume context
	volCtx := make(map[string]string)
	volCtx["subPath"] = subPath
	volCtx["capacity"] = strconv.FormatInt(options.PVC.Spec.Resources.Requests.Storage().Value(), 10)
	for k, v := range scParams {
		volCtx[k] = v
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceStorage): options.PVC.Spec.Resources.Requests[corev1.ResourceName(corev1.ResourceStorage)],
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           config.DriverName,
					VolumeHandle:     pvName,
					ReadOnly:         false,
					FSType:           "juicefs",
					VolumeAttributes: volCtx,
					NodePublishSecretRef: &corev1.SecretReference{
						Name:      scParams[config.PublishSecretName],
						Namespace: scParams[config.PublishSecretNamespace],
					},
				},
			},
			AccessModes:                   options.PVC.Spec.AccessModes,
			PersistentVolumeReclaimPolicy: *options.StorageClass.ReclaimPolicy,
			StorageClassName:              options.StorageClass.Name,
			MountOptions:                  mountOptions,
			VolumeMode:                    options.PVC.Spec.VolumeMode,
		},
	}
	if scParams[config.ControllerExpandSecretName] != "" && scParams[config.ControllerExpandSecretNamespace] != "" {
		pv.Spec.CSI.ControllerExpandSecretRef = &corev1.SecretReference{
			Name:      scParams[config.ControllerExpandSecretName],
			Namespace: scParams[config.ControllerExpandSecretNamespace],
		}
	}

	if pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimDelete && options.StorageClass.Parameters["secretFinalizer"] == "true" {
		klog.V(6).Infof("Provisioner: Add Finalizer on %s/%s", secret.Namespace, secret.Name)
		err = util.AddSecretFinalizer(ctx, j.K8sClient, secret, config.Finalizer)
		if err != nil {
			klog.Warningf("Fails to add a finalizer to the secret, error: %v", err)
		}
	}
	return pv, provisioncontroller.ProvisioningFinished, nil
}

func (j *provisionerService) Delete(ctx context.Context, volume *corev1.PersistentVolume) error {
	klog.V(6).Infof("Provisioner Delete: Volume %v", volume)
	// If it exists and has a `delete` value, delete the directory.
	// If it exists and has a `retain` value, safe the directory.
	policy := volume.Spec.PersistentVolumeReclaimPolicy
	if policy != corev1.PersistentVolumeReclaimDelete {
		klog.V(6).Infof("Provisioner: Volume %s retain, return.", volume.Name)
		return nil
	}
	// check all pvs of the same storageClass, if multiple pv using the same subPath, do not delete the subPath
	shouldDeleted, err := util.CheckForSubPath(ctx, j.K8sClient, volume, volume.Spec.CSI.VolumeAttributes["pathPattern"])
	if err != nil {
		klog.Errorf("Provisioner: CheckForSubPath error: %v", err)
		return err
	}
	if !shouldDeleted {
		klog.Infof("Provisioner: there are other pvs using the same subPath retained, volume %s should not be deleted, return.", volume.Name)
		return nil
	}
	klog.V(6).Infof("Provisioner: there are no other pvs using the same subPath, volume %s can be deleted.", volume.Name)
	subPath := volume.Spec.PersistentVolumeSource.CSI.VolumeAttributes["subPath"]
	secretName, secretNamespace := volume.Spec.CSI.NodePublishSecretRef.Name, volume.Spec.CSI.NodePublishSecretRef.Namespace
	secret, err := j.K8sClient.GetSecret(ctx, secretName, secretNamespace)
	if err != nil {
		klog.Errorf("Provisioner: Get Secret error: %v", err)
		return err
	}
	secretData := make(map[string]string)
	for k, v := range secret.Data {
		secretData[k] = string(v)
	}

	klog.V(5).Infof("Provisioner Delete: Deleting volume subpath %q", subPath)
	if err := j.juicefs.JfsDeleteVol(ctx, volume.Name, subPath, secretData, volume.Spec.CSI.VolumeAttributes, volume.Spec.MountOptions); err != nil {
		klog.Errorf("provisioner: delete vol error %v", err)
		return errors.New("unable to provision delete volume: " + err.Error())
	}

	if volume.Spec.CSI.VolumeAttributes["secretFinalizer"] == "true" {
		shouldRemoveFinalizer, err := util.CheckForSecretFinalizer(ctx, j.K8sClient, volume)
		if err != nil {
			klog.Errorf("Provisioner: CheckForSecretFinalizer error: %v", err)
			return err
		}
		if shouldRemoveFinalizer {
			klog.V(6).Infof("Provisioner: Remove Finalizer on %s/%s", secretNamespace, secretName)
			util.RemoveSecretFinalizer(ctx, j.K8sClient, secret, config.Finalizer)
		}
	}
	return nil
}

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
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	provisioncontroller "sigs.k8s.io/sig-storage-lib-external-provisioner/v10/controller"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/dispatch"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
	snapclientset "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
)

var (
	provisionerLog = klog.NewKlogr().WithName("provisioner")
)

type provisionerService struct {
	juicefs juicefs.Interface
	*k8s.K8sClient
	leaderElection              bool
	leaderElectionNamespace     string
	snapClient                  *snapclientset.Clientset
	leaderElectionLeaseDuration time.Duration
	metrics                     *provisionerMetrics
	quotaPool                   *dispatch.Pool
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
	var snapClient *snapclientset.Clientset
	var err error
	if k8sClient != nil && k8sClient.RestConfig != nil {
		snapClient, err = snapclientset.NewForConfig(k8sClient.RestConfig)
		if err != nil {
			provisionerLog.Error(err, "cannot create snapshot client")
		}
	}
	metrics := newProvisionerMetrics(reg)
	return provisionerService{
		juicefs:                     jfs,
		K8sClient:                   k8sClient,
		leaderElection:              leaderElection,
		leaderElectionNamespace:     leaderElectionNamespace,
		leaderElectionLeaseDuration: leaderElectionLeaseDuration,
		metrics:                     metrics,
		quotaPool:                   dispatch.NewPool(defaultQuotaPoolNum),
		snapClient:                  snapClient,
	}, nil
}

func (j *provisionerService) Run(ctx context.Context) {
	if j.K8sClient == nil {
		provisionerLog.Info("K8sClient is nil")
		os.Exit(1)
	}
	pc := provisioncontroller.NewProvisionController(
		provisionerLog,
		j.K8sClient,
		config.DriverName,
		j,
		provisioncontroller.LeaderElection(j.leaderElection),
		provisioncontroller.LeaseDuration(j.leaderElectionLeaseDuration),
		provisioncontroller.LeaderElectionNamespace(j.leaderElectionNamespace),
	)
	pc.Run(ctx)
}

func (j *provisionerService) setQuotaInProvisioner(
	ctx context.Context,
	volumeId string,
	quota int64,
	mountOptions []string,
	subPath string,
	secrets map[string]string,
	volCtx map[string]string) error {
	log := klog.NewKlogr().WithName("setQuotaInProvisioner")
	subdir := util.ParseSubdirFromMountOptions(mountOptions)
	quotaPath := path.Join("/", subdir, subPath)
	if quota > 0 {
		log.V(1).Info("setting quota in provisioner", "volumeId", volumeId, "name", secrets["name"], "path", quotaPath, "capacity", quota)

		settings, err := j.juicefs.Settings(ctx, volumeId, volumeId, secrets["name"], secrets, volCtx, mountOptions)
		if err != nil {
			log.Error(err, "failed to get settings for quota")
			return status.Errorf(codes.Internal, "Could not get settings for quota: %v", err)
		}

		if err := j.juicefs.SetQuota(ctx, secrets, settings, quotaPath, quota); err != nil {
			log.Error(err, "failed to set quota in provisioner", "quotaPath", quotaPath, "capacity", quota)
			return status.Errorf(codes.Internal, "Could not set quota: %v", err)
		}
	}
	return nil
}

func (j *provisionerService) Provision(ctx context.Context, options provisioncontroller.ProvisionOptions) (*corev1.PersistentVolume, provisioncontroller.ProvisioningState, error) {
	provisionerLog.V(1).Info("provision options", "options", options)
	if options.PVC.Spec.Selector != nil {
		return nil, provisioncontroller.ProvisioningFinished, fmt.Errorf("claim Selector is not supported")
	}

	pvMeta := resource.NewObjectMeta(*options.PVC, options.SelectedNode)

	pvName := options.PVName
	scParams := make(map[string]string)
	for k, v := range options.StorageClass.Parameters {
		if strings.HasPrefix(k, "csi.storage.k8s.io/") {
			scParams[k] = pvMeta.ResolveSecret(v, pvName)
		} else {
			scParams[k] = pvMeta.StringParser(options.StorageClass.Parameters[k])
		}
	}
	provisionerLog.V(1).Info("Resolved StorageClass.Parameters", "params", scParams)

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
				provisionerLog.Info("Volume is set readonly, please make sure the subpath exists.", "subPath", subPath)
			}
		}
	}

	mountOptions := make([]string, 0)
	for _, mo := range options.StorageClass.MountOptions {
		parsedStr := pvMeta.StringParser(mo)
		mountOptions = append(mountOptions, strings.Split(strings.TrimSpace(parsedStr), ",")...)
	}
	provisionerLog.V(1).Info("Resolved MountOptions", "options", mountOptions)

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
						Name:      scParams[common.PublishSecretName],
						Namespace: scParams[common.PublishSecretNamespace],
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
	if scParams[common.ControllerExpandSecretName] != "" && scParams[common.ControllerExpandSecretNamespace] != "" {
		pv.Spec.CSI.ControllerExpandSecretRef = &corev1.SecretReference{
			Name:      scParams[common.ControllerExpandSecretName],
			Namespace: scParams[common.ControllerExpandSecretNamespace],
		}
	}

	if pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimDelete && options.StorageClass.Parameters["secretFinalizer"] == "true" {
		secret, err := j.K8sClient.GetSecret(ctx, scParams[common.PublishSecretName], scParams[common.PublishSecretNamespace])
		if err != nil {
			provisionerLog.Error(err, "Get Secret error")
			j.metrics.provisionErrors.Inc()
			return nil, provisioncontroller.ProvisioningFinished, errors.New("unable to provision new pv: " + err.Error())
		}

		provisionerLog.V(1).Info("Add Finalizer", "namespace", secret.Namespace, "name", secret.Name)
		err = resource.AddSecretFinalizer(ctx, j.K8sClient, secret, common.Finalizer)
		if err != nil {
			provisionerLog.Error(err, "Fails to add a finalizer to the secret")
		}
	}

	if config.GlobalConfig.EnableSetQuota == nil || *config.GlobalConfig.EnableSetQuota {
		if config.GlobalConfig.EnableControllerSetQuota == nil || *config.GlobalConfig.EnableControllerSetQuota {
			if util.SupportQuotaPathCreate(true, config.BuiltinCeVersion) && util.SupportQuotaPathCreate(false, config.BuiltinEeVersion) {
				secret, err := j.K8sClient.GetSecret(ctx, scParams[common.ControllerExpandSecretName], scParams[common.ControllerExpandSecretNamespace])
				if err == nil {
					secretData := make(map[string]string)
					for k, v := range secret.Data {
						secretData[k] = string(v)
					}
					volCtx[common.ControllerQuotaSetKey] = "true"
					cap := options.PVC.Spec.Resources.Requests.Storage().Value()
					j.quotaPool.Run(context.Background(), func(ctx context.Context) {
						if err := j.setQuotaInProvisioner(ctx, pvName, cap, mountOptions, subPath, secretData, volCtx); err != nil {
							provisionerLog.Error(err, "set quota in provisioner error")
						}
					})
				}
			}
		}
	}

	if options.PVC.Spec.DataSource != nil {
		if j.snapClient == nil {
			provisionerLog.Error(errors.New("snapshot client is nil"), "cannot restore data source")
			return pv, provisioncontroller.ProvisioningFinished, nil
		}
		if err := j.RestoreDataSource(ctx, options.PVC, pv, options.PVC.Spec.DataSource, scParams); err != nil {
			j.metrics.provisionErrors.Inc()
			return nil, provisioncontroller.ProvisioningFinished, fmt.Errorf("error restoring data source: %v", err)
		}
		return pv, provisioncontroller.ProvisioningInBackground, nil
	}
	return pv, provisioncontroller.ProvisioningFinished, nil
}

func (j *provisionerService) RestoreDataSource(ctx context.Context, pvc *corev1.PersistentVolumeClaim, pv *corev1.PersistentVolume, source *corev1.TypedLocalObjectReference, scParams map[string]string) error {
	if source.Kind != "VolumeSnapshot" {
		j.metrics.provisionErrors.Inc()
		return fmt.Errorf("only VolumeSnapshot data source is supported, got %s", source.Kind)
	}
	snapshotObj, err := j.snapClient.SnapshotV1().VolumeSnapshots(pvc.Namespace).Get(ctx, source.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting snapshot %s/%s from api server: %s", pvc.Namespace, source.Name, err)
	}
	if snapshotObj.Status == nil || snapshotObj.Status.BoundVolumeSnapshotContentName == nil {
		return fmt.Errorf("snapshot %s/%s is not ready to use", pvc.Namespace, source.Name)
	}

	snapContentObj, err := j.snapClient.SnapshotV1().VolumeSnapshotContents().Get(ctx, *snapshotObj.Status.BoundVolumeSnapshotContentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting snapshotcontent %s for snapshot %s/%s from api server: %s", *snapshotObj.Status.BoundVolumeSnapshotContentName, snapshotObj.Namespace, snapshotObj.Name, err)
	}

	if snapContentObj.Spec.VolumeSnapshotRef.UID != snapshotObj.UID || snapContentObj.Spec.VolumeSnapshotRef.Namespace != snapshotObj.Namespace || snapContentObj.Spec.VolumeSnapshotRef.Name != snapshotObj.Name {
		return fmt.Errorf("snapshotcontent %s for snapshot %s/%s is bound to a different snapshot", *snapshotObj.Status.BoundVolumeSnapshotContentName, snapshotObj.Namespace, snapshotObj.Name)
	}

	if snapshotObj.Status.ReadyToUse == nil || !*snapshotObj.Status.ReadyToUse {
		return fmt.Errorf("snapshot %s is not Ready", source.Name)
	}
	if snapContentObj.Status == nil || snapContentObj.Status.SnapshotHandle == nil {
		return fmt.Errorf("snapshot handle %s is not available", source.Name)
	}

	snapshotId, sourceVolumeId, err := util.ParseSnapshotHandle(*snapContentObj.Status.SnapshotHandle)
	if err != nil {
		return fmt.Errorf("parse snapshot handle %s error: %v", *snapContentObj.Status.SnapshotHandle, err)
	}
	targetVolumeID := pv.Spec.CSI.VolumeHandle
	targetSubPath := pv.Spec.CSI.VolumeAttributes["subPath"]
	provisionerLog.Info("Restoring volume from snapshot", "snapshotId", snapshotId, "sourceVolumeId", sourceVolumeId, "targetVolumeID", targetVolumeID, "targetSubPath", targetSubPath)
	secrets := make(map[string]string)
	secretName := pv.Spec.CSI.NodePublishSecretRef.Name
	secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Namespace
	if scParams[common.ProvisionerSecretName] != "" && scParams[common.ProvisionerSecretNamespace] != "" {
		secretName = scParams[common.ProvisionerSecretName]
		secretNamespace = scParams[common.ProvisionerSecretNamespace]
	}
	secret, err := j.K8sClient.GetSecret(ctx, secretName, secretNamespace)
	if err != nil {
		return fmt.Errorf("get secret %s/%s error: %v", secretNamespace, secretName, err)
	}
	for k, v := range secret.Data {
		secrets[k] = string(v)
	}
	volCtx := pv.Spec.CSI.VolumeAttributes
	return j.juicefs.RestoreSnapshot(ctx, snapshotId, sourceVolumeId, targetVolumeID, targetSubPath, secrets, volCtx)
}

func (j *provisionerService) Delete(ctx context.Context, volume *corev1.PersistentVolume) error {
	provisionerLog.V(1).Info("Delete volume", "volume", *volume)
	// If it exists and has a `delete` value, delete the directory.
	// If it exists and has a `retain` value, safe the directory.
	policy := volume.Spec.PersistentVolumeReclaimPolicy
	if policy != corev1.PersistentVolumeReclaimDelete {
		provisionerLog.V(1).Info("Volume retain, return.", "volume", volume.Name)
		return nil
	}
	// check all pvs of the same storageClass, if multiple pv using the same subPath, do not delete the subPath
	shouldDeleted, err := resource.CheckForSubPath(ctx, j.K8sClient, volume, volume.Spec.CSI.VolumeAttributes["pathPattern"])
	if err != nil {
		provisionerLog.Error(err, "check for subPath error")
		return err
	}
	if !shouldDeleted {
		provisionerLog.Info("there are other pvs using the same subPath retained, volume should not be deleted, return.", "volume", volume.Name)
		return nil
	}
	provisionerLog.V(1).Info("there are no other pvs using the same subPath, volume can be deleted.", "volume", volume.Name)
	subPath := volume.Spec.PersistentVolumeSource.CSI.VolumeAttributes["subPath"]
	secretName, secretNamespace := volume.Spec.CSI.NodePublishSecretRef.Name, volume.Spec.CSI.NodePublishSecretRef.Namespace
	secret, err := j.K8sClient.GetSecret(ctx, secretName, secretNamespace)
	if err != nil {
		provisionerLog.Error(err, "Get Secret error")
		return err
	}
	secretData := make(map[string]string)
	for k, v := range secret.Data {
		secretData[k] = string(v)
	}

	provisionerLog.Info("Deleting volume subpath", "subPath", subPath)
	if err := j.juicefs.JfsDeleteVol(ctx, volume.Name, subPath, secretData, volume.Spec.CSI.VolumeAttributes, volume.Spec.MountOptions); err != nil {
		provisionerLog.Error(err, "delete vol error")
		return errors.New("unable to provision delete volume: " + err.Error())
	}

	if volume.Spec.CSI.VolumeAttributes["secretFinalizer"] == "true" {
		shouldRemoveFinalizer, err := resource.CheckForSecretFinalizer(ctx, j.K8sClient, volume)
		if err != nil {
			provisionerLog.Error(err, "CheckForSecretFinalizer error")
			return err
		}
		if shouldRemoveFinalizer {
			provisionerLog.V(1).Info("Remove Finalizer", "namespace", secretNamespace, "name", secretName)
			if err = resource.RemoveSecretFinalizer(ctx, j.K8sClient, secret, common.Finalizer); err != nil {
				return err
			}
		}
	}
	return nil
}

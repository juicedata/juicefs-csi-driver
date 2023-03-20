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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	provisioncontroller "sigs.k8s.io/sig-storage-lib-external-provisioner/v6/controller"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type provisionerService struct {
	juicefs juicefs.Interface
	*k8s.K8sClient
}

func newProvisionerService(k8sClient *k8s.K8sClient) (provisionerService, error) {
	if k8sClient == nil {
		return provisionerService{}, errors.New("k8sClient is nil")
	}
	jfs := juicefs.NewJfsProvider(nil, k8sClient)
	return provisionerService{
		juicefs:   jfs,
		K8sClient: k8sClient,
	}, nil
}

func (j *provisionerService) Run(ctx context.Context) {
	serverVersion, err := j.K8sClient.Discovery().ServerVersion()
	if err != nil {
		klog.Fatalf("Error getting server version: %v", err)
	}
	leaderElection := true
	leaderElectionEnv := os.Getenv("ENABLE_LEADER_ELECTION")
	if leaderElectionEnv != "" {
		leaderElection, err = strconv.ParseBool(leaderElectionEnv)
		if err != nil {
			klog.Fatalf("Unable to parse ENABLE_LEADER_ELECTION env var: %v", err)
		}
	}
	pc := provisioncontroller.NewProvisionController(j.K8sClient,
		config.DriverName,
		j,
		serverVersion.GitVersion,
		provisioncontroller.LeaderElection(leaderElection),
	)
	pc.Run(ctx)
}

func (j *provisionerService) Provision(ctx context.Context, options provisioncontroller.ProvisionOptions) (*corev1.PersistentVolume, provisioncontroller.ProvisioningState, error) {
	klog.V(6).Infof("Provisioner Provision: options %v", options)
	if options.PVC.Spec.Selector != nil {
		return nil, provisioncontroller.ProvisioningFinished, fmt.Errorf("claim Selector is not supported")
	}

	pvName := options.PVName
	sc := options.StorageClass

	pvMeta := util.NewPVCMeta(*options.PVC)
	subPath := options.PVName
	if options.StorageClass.Parameters["pathPattern"] != "" {
		subPath = pvMeta.StringParser(options.StorageClass.Parameters["pathPattern"])
	}
	secretName, secretNamespace := sc.Parameters[config.ProvisionerSecretName], sc.Parameters[config.ProvisionerSecretNamespace]
	secret, err := j.K8sClient.GetSecret(ctx, secretName, secretNamespace)
	if err != nil {
		klog.Errorf("[PVCReconciler]: Get Secret error: %v", err)
		return nil, provisioncontroller.ProvisioningFinished, errors.New("unable to provision new pv: " + err.Error())
	}
	secretData := make(map[string]string)
	for k, v := range secret.Data {
		secretData[k] = string(v)
	}
	// set volume context
	volCtx := make(map[string]string)
	volCtx["subPath"] = subPath
	for k, v := range sc.Parameters {
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
						Name:      sc.Parameters[config.PublishSecretName],
						Namespace: sc.Parameters[config.PublishSecretNamespace],
					},
				},
			},
			AccessModes:                   options.PVC.Spec.AccessModes,
			PersistentVolumeReclaimPolicy: *options.StorageClass.ReclaimPolicy,
			StorageClassName:              sc.Name,
			MountOptions:                  options.StorageClass.MountOptions,
			VolumeMode:                    options.PVC.Spec.VolumeMode,
		},
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
	secretName, secretNamespace := volume.Spec.CSI.VolumeAttributes[config.ProvisionerSecretName], volume.Spec.CSI.VolumeAttributes[config.ProvisionerSecretNamespace]
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
	if err := j.juicefs.JfsDeleteVol(ctx, volume.Name, subPath, secretData, volume.Spec.CSI.VolumeAttributes); err != nil {
		klog.Errorf("provisioner: delete vol error %v", err)
		return errors.New("unable to provision delete volume: " + err.Error())
	}

	return nil
}

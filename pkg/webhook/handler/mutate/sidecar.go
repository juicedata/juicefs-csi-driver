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

package mutate

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type SidecarMutate struct {
	Client  *k8sclient.K8sClient
	juicefs juicefs.Interface

	PVC        *corev1.PersistentVolumeClaim
	PV         *corev1.PersistentVolume
	jfsSetting *config.JfsSetting
}

var _ Mutate = &SidecarMutate{}

func NewSidecarMutate(client *k8sclient.K8sClient, jfs juicefs.Interface, pvc *corev1.PersistentVolumeClaim, pv *corev1.PersistentVolume) Mutate {
	return &SidecarMutate{
		Client:  client,
		juicefs: jfs,
		PVC:     pvc,
		PV:      pv,
	}
}

func (s *SidecarMutate) Mutate(pod *corev1.Pod) (out *corev1.Pod, err error) {
	// get secret, volumeContext and mountOptions from PV
	secrets, volCtx, options, err := s.GetSettings(*s.PV)
	if err != nil {
		klog.Infof("get settings from pv %s of pod %s namespace %s err: %v", s.PV.Name, pod.Name, pod.Namespace, err)
		return
	}
	klog.V(6).Infof("secrets: %v, volumeContext: %v, mountOptions: %v", secrets, volCtx, options)

	out = pod.DeepCopy()
	// gen jfs settings
	jfsSetting, err := s.juicefs.Settings(context.TODO(), s.PV.Spec.CSI.VolumeHandle, secrets, volCtx, options)
	if err != nil {
		return
	}
	jfsSetting.MountPath = filepath.Join(config.PodMountBase, jfsSetting.VolumeId)

	jfsSetting.Attr.Namespace = pod.Namespace
	jfsSetting.SecretName = s.PVC.Name + "-jfs-secret"
	s.jfsSetting = jfsSetting
	r := builder.NewBuilder(jfsSetting)

	// create secret per PVC
	secret := r.NewSecret()
	builder.SetPVCAsOwner(&secret, s.PVC)
	if err = s.createOrUpdateSecret(context.TODO(), &secret); err != nil {
		return
	}

	// gen mount pod
	mountPod := r.NewMountSidecar()
	podStr, _ := json.Marshal(mountPod)
	klog.V(6).Infof("mount pod: %v\n", string(podStr))
	// inject container
	s.injectContainer(out, mountPod.Spec.Containers[0])
	// inject initContainer
	s.injectInitContainer(out, mountPod.Spec.InitContainers[0])
	// inject volume
	s.injectVolume(out, mountPod.Spec.Volumes)

	return
}

func (s *SidecarMutate) GetSettings(pv corev1.PersistentVolume) (secrets, volCtx map[string]string, options []string, err error) {
	// get secret
	secret, err := s.Client.GetSecret(
		context.TODO(),
		pv.Spec.CSI.NodePublishSecretRef.Name,
		pv.Spec.CSI.NodePublishSecretRef.Namespace,
	)
	if err != nil {
		return
	}

	secrets = make(map[string]string)
	for k, v := range secret.Data {
		secrets[k] = string(v)
	}
	volCtx = pv.Spec.CSI.VolumeAttributes
	klog.V(5).Infof("volume context of pv %s: %v", pv.Name, volCtx)

	options = []string{}
	if len(pv.Spec.AccessModes) == 1 && pv.Spec.AccessModes[0] == corev1.ReadOnlyMany {
		options = append(options, "ro")
	}
	// get mountOptions from PV.spec.mountOptions
	options = append(options, pv.Spec.MountOptions...)

	mountOptions := []string{}
	// get mountOptions from PV.volumeAttributes
	if opts, ok := volCtx["mountOptions"]; ok {
		mountOptions = strings.Split(opts, ",")
	}
	options = append(options, mountOptions...)

	return
}

func (s *SidecarMutate) injectContainer(pod *corev1.Pod, container corev1.Container) {
	pod.Spec.Containers = append([]corev1.Container{container}, pod.Spec.Containers...)
}

func (s *SidecarMutate) injectInitContainer(pod *corev1.Pod, container corev1.Container) {
	pod.Spec.InitContainers = append([]corev1.Container{container}, pod.Spec.InitContainers...)
}

func (s *SidecarMutate) injectVolume(pod *corev1.Pod, volumes []corev1.Volume) {
	hostMount := filepath.Join(config.MountPointPath, s.jfsSetting.VolumeId)
	for i, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == s.PVC.Name {
			// overwrite original volume and use juicefs volume mountpoint instead
			pod.Spec.Volumes[i] = corev1.Volume{
				Name: volume.Name,
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: hostMount,
					},
				}}
		}
	}
	// inject volume
	pod.Spec.Volumes = append(pod.Spec.Volumes, volumes...)
}

func (s *SidecarMutate) createOrUpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	klog.V(5).Infof("createOrUpdateSecret: %s, %s", secret.Name, secret.Namespace)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		oldSecret, err := s.Client.GetSecret(ctx, secret.Name, secret.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// secret not exist, create
				_, err := s.Client.CreateSecret(ctx, secret)
				return err
			}
			// unexpected err
			return err
		}

		oldSecret.StringData = secret.StringData
		return s.Client.UpdateSecret(ctx, oldSecret)
	})
	if err != nil {
		klog.Errorf("createOrUpdateSecret: secret %s: %v", secret.Name, err)
		return err
	}
	return nil
}

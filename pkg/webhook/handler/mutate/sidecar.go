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
	"fmt"
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
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type SidecarMutate struct {
	Client     *k8sclient.K8sClient
	juicefs    juicefs.Interface
	Serverless bool

	Pair       []util.PVPair
	jfsSetting *config.JfsSetting
}

var _ Mutate = &SidecarMutate{}

func NewSidecarMutate(client *k8sclient.K8sClient, jfs juicefs.Interface, serverless bool, pair []util.PVPair) Mutate {
	return &SidecarMutate{
		Client:     client,
		juicefs:    jfs,
		Serverless: serverless,
		Pair:       pair,
	}
}

func (s *SidecarMutate) Mutate(ctx context.Context, pod *corev1.Pod) (out *corev1.Pod, err error) {
	out = pod.DeepCopy()
	for i, pair := range s.Pair {
		out, err = s.mutate(ctx, out, pair, i)
		if err != nil {
			return
		}
	}
	return
}

func (s *SidecarMutate) mutate(ctx context.Context, pod *corev1.Pod, pair util.PVPair, index int) (out *corev1.Pod, err error) {
	// get secret, volumeContext and mountOptions from PV
	secrets, volCtx, options, err := s.GetSettings(*pair.PV)
	if err != nil {
		klog.Errorf("get settings from pv %s of pod %s namespace %s err: %v", pair.PV.Name, pod.Name, pod.Namespace, err)
		return
	}

	// overwrite volume context
	for k, v := range pair.PVC.Annotations {
		if !strings.HasPrefix(k, "juicefs") {
			continue
		}
		volCtx[k] = v
	}
	out = pod.DeepCopy()
	// gen jfs settings
	jfsSetting, err := s.juicefs.Settings(ctx, pair.PV.Spec.CSI.VolumeHandle, secrets, volCtx, options)
	if err != nil {
		return
	}
	mountPath := util.RandStringRunes(6)
	jfsSetting.MountPath = filepath.Join(config.PodMountBase, mountPath)

	jfsSetting.Attr.Namespace = pod.Namespace
	jfsSetting.SecretName = pair.PVC.Name + "-jfs-secret"
	s.jfsSetting = jfsSetting
	capacity := pair.PVC.Spec.Resources.Requests.Storage().Value()
	cap := capacity / 1024 / 1024 / 1024
	if cap <= 0 {
		return nil, fmt.Errorf("capacity %d is too small, at least 1GiB for quota", capacity)
	}

	var r builder.SidecarInterface
	if !s.Serverless {
		r = builder.NewContainerBuilder(jfsSetting, cap)
	} else if pod.Annotations != nil && pod.Annotations[builder.VCIANNOKey] == builder.VCIANNOValue {
		r = builder.NewVCIBuilder(jfsSetting, cap, *pod, *pair.PVC)
	} else if pod.Labels != nil && pod.Labels[builder.CCIANNOKey] == builder.CCIANNOValue {
		r = builder.NewCCIBuilder(jfsSetting, cap, *pod, *pair.PVC)
	} else {
		r = builder.NewServerlessBuilder(jfsSetting, cap)
	}

	// create secret per PVC
	secret := r.NewSecret()
	builder.SetPVCAsOwner(&secret, pair.PVC)
	if err = s.createOrUpdateSecret(ctx, &secret); err != nil {
		return
	}

	// gen mount pod
	mountPod := r.NewMountSidecar()
	podStr, _ := json.Marshal(mountPod)
	klog.V(6).Infof("mount pod: %v\n", string(podStr))

	// deduplicate container name and volume name in pod when multiple volumes are mounted
	s.Deduplicate(pod, mountPod, index)

	// inject volume
	s.injectVolume(out, r, mountPod.Spec.Volumes, mountPath, pair)
	// inject label
	s.injectLabel(out)
	// inject annotation
	s.injectAnnotation(out, mountPod.Annotations)
	// inject container
	s.injectContainer(out, mountPod.Spec.Containers[0])

	return
}

func (s *SidecarMutate) Deduplicate(pod, mountPod *corev1.Pod, index int) {
	// deduplicate container name
	for _, c := range pod.Spec.Containers {
		if c.Name == mountPod.Spec.Containers[0].Name {
			mountPod.Spec.Containers[0].Name = fmt.Sprintf("%s-%d", c.Name, index)
		}
	}

	// deduplicate volume name
	for i, mv := range mountPod.Spec.Volumes {
		if mv.Name == builder.UpdateDBDirName || mv.Name == builder.JfsDirName {
			continue
		}
		mountIndex := 0
		for j, mm := range mountPod.Spec.Containers[0].VolumeMounts {
			if mm.Name == mv.Name {
				mountIndex = j
				break
			}
		}
		for _, v := range pod.Spec.Volumes {
			if v.Name == mv.Name {
				mountPod.Spec.Volumes[i].Name = fmt.Sprintf("%s-%s", mountPod.Spec.Containers[0].Name, v.Name)
				mountPod.Spec.Containers[0].VolumeMounts[mountIndex].Name = mountPod.Spec.Volumes[i].Name
			}
		}
	}

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
	klog.V(5).Infof("volume options of pv %s: %v", pv.Name, options)

	return
}

func (s *SidecarMutate) injectContainer(pod *corev1.Pod, container corev1.Container) {
	pod.Spec.Containers = append([]corev1.Container{container}, pod.Spec.Containers...)
}

func (s *SidecarMutate) injectVolume(pod *corev1.Pod, build builder.SidecarInterface, volumes []corev1.Volume, mountPath string, pair util.PVPair) {
	mountedVolume := []corev1.Volume{}
	podVolumes := make(map[string]bool)
	for _, volume := range pod.Spec.Volumes {
		podVolumes[volume.Name] = true
	}
	for _, v := range volumes {
		if v.Name == builder.UpdateDBDirName || v.Name == builder.JfsDirName {
			if _, ok := podVolumes[v.Name]; ok {
				continue
			}
		}
		mountedVolume = append(mountedVolume, v)
	}
	for i, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pair.PVC.Name {
			// overwrite volume
			build.OverwriteVolumes(&volume, mountPath)
			pod.Spec.Volumes[i] = volume

			for cni, cn := range pod.Spec.Containers {
				for j, vm := range cn.VolumeMounts {
					// overwrite volumeMount
					if vm.Name == volume.Name {
						build.OverwriteVolumeMounts(&vm)
						pod.Spec.Containers[cni].VolumeMounts[j] = vm
					}
				}
			}
		}
	}
	// inject volume
	pod.Spec.Volumes = append(pod.Spec.Volumes, mountedVolume...)
}

func (s *SidecarMutate) injectLabel(pod *corev1.Pod) {
	metaObj := pod.ObjectMeta

	if metaObj.Labels == nil {
		metaObj.Labels = map[string]string{}
	}

	metaObj.Labels[config.InjectSidecarDone] = config.True
	metaObj.DeepCopyInto(&pod.ObjectMeta)
}

func (s *SidecarMutate) injectAnnotation(pod *corev1.Pod, annotations map[string]string) {
	metaObj := pod.ObjectMeta

	if metaObj.Annotations == nil {
		metaObj.Annotations = map[string]string{}
	}

	for k, v := range annotations {
		metaObj.Annotations[k] = v
	}
	metaObj.DeepCopyInto(&pod.ObjectMeta)
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

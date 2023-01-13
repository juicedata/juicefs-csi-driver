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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook/handler/mutate"
)

type SidecarHandler struct {
	Client *k8sclient.K8sClient
	// A decoder will be automatically injected
	decoder *admission.Decoder
}

func (s *SidecarHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	pod := &corev1.Pod{}
	raw := request.Object.Raw
	klog.V(6).Infof("[SidecarHandler] get pod: %s", string(raw))
	err := s.decoder.Decode(request, pod)
	if err != nil {
		klog.Error(err, "unable to decoder pod from req")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// check if pod has done label
	if util.CheckExpectValue(pod.Labels, config.InjectSidecarDone, config.True) {
		klog.Infof("[SidecarHandler] skip mutating the pod because injection is done. Pod %s namespace %s", pod.Name, pod.Namespace)
		return admission.Allowed("skip mutating the pod because injection is done")
	}

	// check if pod has disable label
	if util.CheckExpectValue(pod.Labels, config.InjectSidecarDisable, config.True) {
		klog.Infof("[SidecarHandler] skip mutating the pod because injection is disabled. Pod %s namespace %s", pod.Name, pod.Namespace)
		return admission.Allowed("skip mutating the pod because injection is disabled")
	}

	// check if pod use JuiceFS Volume
	used, pv, pvc, err := s.GetVolume(ctx, *pod)
	if err != nil {
		klog.Errorf("[SidecarHandler] get pv from pod %s namespace %s err: %v", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusBadRequest, err)
	} else if !used {
		klog.Infof("[SidecarHandler] skip mutating the pod because it doesn't use JuiceFS Volume. Pod %s namespace %s", pod.Name, pod.Namespace)
		return admission.Allowed("skip mutating the pod because it doesn't use JuiceFS Volume")
	}

	jfs := juicefs.NewJfsProvider(nil, s.Client)
	sidecarMutate := mutate.NewSidecarMutate(s.Client, jfs, pvc, pv)
	klog.Infof("[SidecarHandler] start injecting juicefs client as sidecar in pod [%s] namespace [%s].", pod.Name, pod.Namespace)
	out, err := sidecarMutate.Mutate(ctx, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	marshaledPod, err := json.Marshal(out)
	if err != nil {
		klog.Error(err, "unable to marshal pod")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	resp := admission.PatchResponseFromRaw(raw, marshaledPod)
	return resp
}

// InjectDecoder injects the decoder.
func (s *SidecarHandler) InjectDecoder(d *admission.Decoder) error {
	s.decoder = d
	return nil
}

// GetVolume get juicefs pv & pvc from pod
func (s *SidecarHandler) GetVolume(ctx context.Context, pod corev1.Pod) (used bool, pvGot *corev1.PersistentVolume, pvcGot *corev1.PersistentVolumeClaim, err error) {
	klog.V(6).Infof("Volumes of pod %s: %v", pod.Name, pod.Spec.Volumes)
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			// get PVC
			var pvc *corev1.PersistentVolumeClaim
			pvc, err = s.Client.GetPersistentVolumeClaim(ctx, volume.PersistentVolumeClaim.ClaimName, pod.Namespace)
			if err != nil {
				return
			}

			// get storageclass
			if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
				var sc *storagev1.StorageClass
				sc, err = s.Client.GetStorageClass(ctx, *pvc.Spec.StorageClassName)
				if err != nil {
					return
				}
				// if storageclass is juicefs
				if sc.Provisioner == config.DriverName {
					used = true
					pvcGot = pvc
				}
			}

			// get PV
			var pv *corev1.PersistentVolume
			if pvc.Spec.VolumeName == "" {
				if used {
					// used juicefs volume, but pvc is not bound
					err = fmt.Errorf("pvc %s is not bound", pvc.Name)
					return
				}
				continue
			}
			pv, err = s.Client.GetPersistentVolume(ctx, pvc.Spec.VolumeName)
			if err != nil {
				return
			}
			// if PV is JuiceFS PV
			if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
				used = true
				pvGot = pv
				pvcGot = pvc
				return
			}
		}
	}
	return
}

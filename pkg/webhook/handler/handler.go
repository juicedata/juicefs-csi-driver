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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	used, pair, err := s.GetVolumes(ctx, *pod)
	if err != nil {
		klog.Errorf("[SidecarHandler] get pv from pod %s namespace %s err: %v", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusBadRequest, err)
	} else if !used {
		klog.Infof("[SidecarHandler] skip mutating the pod because it doesn't use JuiceFS Volume. Pod %s namespace %s", pod.Name, pod.Namespace)
		return admission.Allowed("skip mutating the pod because it doesn't use JuiceFS Volume")
	}

	jfs := juicefs.NewJfsProvider(nil, s.Client)
	for _, pvPair := range pair {
		sidecarMutate := mutate.NewSidecarMutate(s.Client, jfs, pvPair.PVC, pvPair.PV)
		klog.Infof("[SidecarHandler] start injecting juicefs client as sidecar in pod [%s] namespace [%s].", pod.Name, pod.Namespace)
		out, err := sidecarMutate.Mutate(ctx, pod)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		pod = out
	}

	marshaledPod, err := json.Marshal(pod)
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

type PVPair struct {
	PV  *corev1.PersistentVolume
	PVC *corev1.PersistentVolumeClaim
}

// GetVolumes get juicefs pv & pvc from pod
func (s *SidecarHandler) GetVolumes(ctx context.Context, pod corev1.Pod) (used bool, pvPairGot []PVPair, err error) {
	klog.V(6).Infof("Volumes of pod %s: %v", pod.Name, pod.Spec.Volumes)
	var (
		namespace = pod.Namespace
	)
	if pod.OwnerReferences == nil && namespace == "" {
		// if pod is not created by controller, namespace is empty, set default namespace
		namespace = "default"
	}
	pvPairGot = []PVPair{}
	if namespace == "" {
		pvPairGot, err = s.getVolWithoutNamespace(ctx, pod)
		if err != nil {
			return
		}
		used = len(pvPairGot) != 0
		return
	}
	pvPairGot, err = s.getVolWithNamespace(ctx, pod, namespace)
	used = len(pvPairGot) != 0
	return
}

func (s *SidecarHandler) getVolWithNamespace(ctx context.Context, pod corev1.Pod, namespace string) (pvPairGot []PVPair, err error) {
	used := false
	pvPairGot = []PVPair{}
	// if namespace is got from pod
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			// get PVC
			var pvc *corev1.PersistentVolumeClaim
			pvc, err = s.Client.GetPersistentVolumeClaim(ctx, volume.PersistentVolumeClaim.ClaimName, namespace)
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
				pvPairGot = append(pvPairGot, PVPair{PV: pv, PVC: pvc})
			}
		}
	}
	return
}

// getVolWithoutNamespace get juicefs pv & pvc from pod when pod namespace is empty
func (s *SidecarHandler) getVolWithoutNamespace(ctx context.Context, pod corev1.Pod) (pair []PVPair, err error) {
	pair = []PVPair{}
	pvs, err := s.Client.ListPersistentVolumes(ctx, nil, nil)
	if err != nil {
		return
	}
	juicePVs := []corev1.PersistentVolume{}
	for _, pv := range pvs {
		if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != config.DriverName {
			// skip if PV is not JuiceFS PV
			continue
		}
		juicePVs = append(juicePVs, pv)
	}
	pairs := []PVPair{}
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcName := volume.PersistentVolumeClaim.ClaimName
			for _, pv := range juicePVs {
				if pv.Spec.ClaimRef.Name == pvcName {
					if pv.Spec.ClaimRef == nil {
						err = fmt.Errorf("pvc %s is not bound", pvcName)
						return
					}
					// check if pod's owner in the same namespace
					if e := s.checkOwner(ctx, pod.OwnerReferences, pv.Spec.ClaimRef.Namespace); e != nil {
						if errors.IsNotFound(e) {
							// pod's owner is not in the same namespace, skip
							continue
						}
						err = e
						return
					}
					// get PVC
					var pvc *corev1.PersistentVolumeClaim
					pvc, err = s.Client.GetPersistentVolumeClaim(ctx, volume.PersistentVolumeClaim.ClaimName, pv.Spec.ClaimRef.Namespace)
					if err != nil {
						return
					}
					pairs = append(pairs, PVPair{
						PV:  &pv,
						PVC: pvc,
					})
				}
			}
		}
	}

	return
}

func (s *SidecarHandler) checkOwner(ctx context.Context, owner []metav1.OwnerReference, namespace string) error {
	for _, o := range owner {
		if o.Kind == "ReplicaSet" {
			_, err := s.Client.GetReplicaSet(ctx, o.Name, namespace)
			return err
		}
		if o.Kind == "StatefulSet" {
			_, err := s.Client.GetStatefulSet(ctx, o.Name, namespace)
			return err
		}
		if o.Kind == "DaemonSet" {
			_, err := s.Client.GetDaemonSet(ctx, o.Name, namespace)
			return err
		}
		if o.Kind == "Job" {
			_, err := s.Client.GetJob(ctx, o.Name, namespace)
			return err
		}
	}
	return fmt.Errorf("no owner found")
}

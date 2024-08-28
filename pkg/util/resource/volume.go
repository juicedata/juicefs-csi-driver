/*
 Copyright 2023 Juicedata Inc

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

package resource

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type PVPair struct {
	PV  *corev1.PersistentVolume
	PVC *corev1.PersistentVolumeClaim
}

// GetVolumes get juicefs pv & pvc from pod
func GetVolumes(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod) (used bool, pvPairGot []PVPair, err error) {
	resourceLog.V(1).Info("Volumes of pod", "podName", pod.Name, "volumes", pod.Spec.Volumes)
	var namespace string
	pvPairGot = []PVPair{}
	namespace, err = GetNamespace(ctx, client, pod)
	if err != nil {
		return
	}
	pod.Namespace = namespace
	pvPairGot, err = getVol(ctx, client, pod, namespace)
	resourceLog.V(1).Info("get pv pair", "pv pair", pvPairGot)
	used = len(pvPairGot) != 0
	return
}

func getVol(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod, namespace string) (pvPairGot []PVPair, err error) {
	pvPairGot = []PVPair{}
	// if namespace is got from pod
	for _, volume := range pod.Spec.Volumes {
		used := false
		if volume.PersistentVolumeClaim != nil {
			// get PVC
			var pvc *corev1.PersistentVolumeClaim
			pvc, err = client.GetPersistentVolumeClaim(ctx, volume.PersistentVolumeClaim.ClaimName, namespace)
			if err != nil {
				return
			}

			// get storageclass
			if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
				var sc *storagev1.StorageClass
				sc, err = client.GetStorageClass(ctx, *pvc.Spec.StorageClassName)
				if err != nil && !k8serrors.IsNotFound(err) {
					// if pvc.storageClassName do not exist, do not return error, and check if it has been bound with static PV.
					return
				}
				// if storageclass is juicefs
				if sc != nil && sc.Provisioner == config.DriverName {
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
			pv, err = client.GetPersistentVolume(ctx, pvc.Spec.VolumeName)
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

type VolumeLocks struct {
	locks sync.Map
	mux   sync.Mutex
}

func NewVolumeLocks() *VolumeLocks {
	return &VolumeLocks{}
}

func (vl *VolumeLocks) TryAcquire(volumeID string) bool {
	vl.mux.Lock()
	defer vl.mux.Unlock()
	if _, ok := vl.locks.Load(volumeID); ok {
		return false
	}
	vl.locks.Store(volumeID, nil)
	return true
}

// Release deletes the lock on volumeID.
func (vl *VolumeLocks) Release(volumeID string) {
	vl.mux.Lock()
	defer vl.mux.Unlock()
	vl.locks.Delete(volumeID)
}

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

package config

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type PVPair struct {
	PV  *corev1.PersistentVolume
	PVC *corev1.PersistentVolumeClaim
}

// GetVolumes get juicefs pv & pvc from pod
func GetVolumes(ctx context.Context, client *k8sclient.K8sClient, pod corev1.Pod) (used bool, pvPairGot []PVPair, err error) {
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
		pvPairGot, err = getVolWithoutNamespace(ctx, client, pod)
		if err != nil {
			return
		}
		used = len(pvPairGot) != 0
		return
	}
	pvPairGot, err = getVolWithNamespace(ctx, client, pod, namespace)
	used = len(pvPairGot) != 0
	return
}

func getVolWithNamespace(ctx context.Context, client *k8sclient.K8sClient, pod corev1.Pod, namespace string) (pvPairGot []PVPair, err error) {
	used := false
	pvPairGot = []PVPair{}
	// if namespace is got from pod
	for _, volume := range pod.Spec.Volumes {
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

// getVolWithoutNamespace get juicefs pv & pvc from pod when pod namespace is empty
func getVolWithoutNamespace(ctx context.Context, client *k8sclient.K8sClient, pod corev1.Pod) (pair []PVPair, err error) {
	pair = []PVPair{}
	pvs, err := client.ListPersistentVolumes(ctx, nil, nil)
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
					if e := checkOwner(ctx, client, pod.OwnerReferences, pv.Spec.ClaimRef.Namespace); e != nil {
						if errors.IsNotFound(e) {
							// pod's owner is not in the same namespace, skip
							continue
						}
						err = e
						return
					}
					// get PVC
					var pvc *corev1.PersistentVolumeClaim
					pvc, err = client.GetPersistentVolumeClaim(ctx, volume.PersistentVolumeClaim.ClaimName, pv.Spec.ClaimRef.Namespace)
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

func checkOwner(ctx context.Context, client *k8sclient.K8sClient, owner []metav1.OwnerReference, namespace string) error {
	for _, o := range owner {
		if o.Kind == "ReplicaSet" {
			_, err := client.GetReplicaSet(ctx, o.Name, namespace)
			return err
		}
		if o.Kind == "StatefulSet" {
			_, err := client.GetStatefulSet(ctx, o.Name, namespace)
			return err
		}
		if o.Kind == "DaemonSet" {
			_, err := client.GetDaemonSet(ctx, o.Name, namespace)
			return err
		}
		if o.Kind == "Job" {
			_, err := client.GetJob(ctx, o.Name, namespace)
			return err
		}
	}
	return fmt.Errorf("no owner found")
}

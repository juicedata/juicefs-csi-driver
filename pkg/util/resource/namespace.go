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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var (
	resourceLog = klog.NewKlogr().WithName("util")
)

// GetNamespace get juicefs pv & pvc from pod when pod namespace is empty
func GetNamespace(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod) (namespace string, err error) {
	namespace = pod.Namespace
	if namespace != "" {
		return
	}
	if pod.OwnerReferences == nil && namespace == "" {
		// if pod is not created by controller, namespace is empty, set default namespace
		namespace = "default"
		return
	}

	// if namespace of pod is empty (see issue: https://github.com/juicedata/juicefs-csi-driver/issues/644), should get namespace from pvc which is used by pod
	// 1. get all juicefs pv
	// 2. get pvc from pod
	// 3. check if pod's owner in the same namespace with pvc
	pvs, err := client.ListPersistentVolumes(ctx, nil, nil)
	if err != nil {
		return
	}
	juicePVs := []corev1.PersistentVolume{}
	for _, pv := range pvs {
		if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != config.DriverName || pv.Spec.ClaimRef == nil {
			// skip if PV is not JuiceFS PV
			continue
		}
		juicePVs = append(juicePVs, pv)
	}
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcName := volume.PersistentVolumeClaim.ClaimName
			for _, pv := range juicePVs {
				if pv.Spec.ClaimRef.Name == pvcName {
					// check if pod's owner in the same namespace
					if e := checkOwner(ctx, client, pod.OwnerReferences, pv.Spec.ClaimRef.Namespace); e != nil {
						if errors.IsNotFound(e) {
							// pod's owner is not in the same namespace, skip
							continue
						}
						err = e
						return
					}
					// get it
					namespace = pv.Spec.ClaimRef.Namespace
					return
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

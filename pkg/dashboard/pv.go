/*
Copyright 2023 The Kubernetes Authors.

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

package dashboard

import (
	"context"
	"log"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (api *API) listPVsOfPod(ctx context.Context, pod *corev1.Pod) (map[string]*corev1.PersistentVolume, error) {
	pvs := make(map[string]*corev1.PersistentVolume)
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		pvc, err := api.k8sClient.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx, v.PersistentVolumeClaim.ClaimName, v1.GetOptions{})
		if err != nil {
			log.Printf("can't get pvc %s/%s: %v\n", pod.Namespace, v.PersistentVolumeClaim.ClaimName, err)
			continue
		}
		pv, err := api.k8sClient.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, v1.GetOptions{})
		if err != nil {
			log.Printf("can't get pv %s: %v\n", pvc.Spec.VolumeName, err)
			continue
		}
		if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != config.DriverName {
			continue
		}
		pvs[v.PersistentVolumeClaim.ClaimName] = pv
	}
	return pvs, nil
}

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

package dashboard

import (
	"context"
	"sort"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (api *API) sysNamespaced(name string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: api.sysNamespace,
		Name:      name,
	}
}

func isAppPod(pod *corev1.Pod) bool {
	if pod.Labels != nil {
		// mount pod mode
		if _, ok := pod.Labels[config.UniqueId]; ok {
			return true
		}
		// sidecar mode
		if _, ok := pod.Labels[config.InjectSidecarDone]; ok {
			return true
		}
	}
	return false
}

func (api *API) isAppPodUnready(ctx context.Context, pod *corev1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcName := types.NamespacedName{Name: volume.PersistentVolumeClaim.ClaimName, Namespace: pod.Namespace}
			if err := api.cachedReader.Get(ctx, pvcName, &corev1.PersistentVolumeClaim{}); err == nil {
				return true
			}
		}
	}
	return false
}

func isSysPod(pod *corev1.Pod) bool {
	if pod.Labels != nil {
		return pod.Labels["app.kubernetes.io/name"] == "juicefs-mount" || pod.Labels["app.kubernetes.io/name"] == "juicefs-csi-driver"
	}
	return false
}

func isCsiNode(pod *corev1.Pod) bool {
	if pod.Labels != nil {
		return pod.Labels["app.kubernetes.io/name"] == "juicefs-csi-driver" && pod.Labels["app"] == "juicefs-csi-node"
	}
	return false
}

type ReverseSort struct {
	sort.Interface
}

func (r *ReverseSort) Less(i, j int) bool {
	return !r.Interface.Less(i, j)
}

func Reverse(data sort.Interface) sort.Interface {
	return &ReverseSort{data}
}

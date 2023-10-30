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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func (api *API) StartManager(ctx context.Context, mgr manager.Manager) error {
	podCtr := PodController{api}
	if err := podCtr.SetupWithManager(mgr); err != nil {
		return err
	}
	return mgr.Start(ctx)
}

type PodController struct {
	*API
}

func (c *PodController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}
	if err := c.cachedReader.Get(ctx, req.NamespacedName, pod); err != nil {
		klog.Errorf("get pod %s failed: %v", req.NamespacedName, err)
		return reconcile.Result{}, nil
	}
	if !isSysPod(pod) && !isAppPod(pod) && !c.isAppPodUnready(ctx, pod) {
		klog.V(6).Infof("pod %s is not required", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	if pod.DeletionTimestamp != nil {
		c.appIndexes.removeIndex(req.NamespacedName)
		return reconcile.Result{}, nil
	}
	indexes := c.appIndexes
	if isSysPod(pod) {
		indexes = c.sysIndexes
		if isCsiNode(pod) {
			c.csiNodeLock.Lock()
			c.csiNodeIndex[pod.Spec.NodeName] = types.NamespacedName{
				Namespace: pod.GetNamespace(),
				Name:      pod.GetName(),
			}
			c.csiNodeLock.Unlock()
		}
	}
	if indexes != nil {
		indexes.addIndex(
			pod,
			func(p *corev1.Pod) metav1.ObjectMeta { return p.ObjectMeta },
			func(name types.NamespacedName) (*corev1.Pod, error) {
				var pod corev1.Pod
				err := c.cachedReader.Get(ctx, name, &pod)
				return &pod, err
			},
		)
	}
	return reconcile.Result{}, nil
}

func (c *PodController) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("pod", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	return ctr.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			pod := event.Object.(*corev1.Pod)
			klog.V(6).Infof("watch pod %s/%s created", pod.GetNamespace(), pod.GetName())
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			pod := deleteEvent.Object.(*corev1.Pod)
			klog.V(6).Infof("watch pod %s%s deleted", pod.GetNamespace(), pod.GetName())
			var indexes *timeOrderedIndexes[corev1.Pod]
			if isAppPod(pod) {
				indexes = c.appIndexes
			} else if isSysPod(pod) {
				indexes = c.sysIndexes
			}
			if indexes != nil {
				indexes.removeIndex(types.NamespacedName{
					Namespace: pod.GetNamespace(),
					Name:      pod.GetName(),
				})
				return false
			}
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	})
}

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

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	pvCtr := PVController{api}
	if err := podCtr.SetupWithManager(mgr); err != nil {
		return err
	}
	if err := pvCtr.SetupWithManager(mgr); err != nil {
		return err
	}
	return mgr.Start(ctx)
}

type PodController struct {
	*API
}

type PVController struct {
	*API
}

type PVCController struct {
	*API
}

func (c *PodController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}
	if err := c.cachedReader.Get(ctx, req.NamespacedName, pod); err != nil {
		klog.Errorf("get pod %s failed: %v", req.NamespacedName, err)
		return reconcile.Result{}, nil
	}
	if !isSysPod(pod) && !isAppPod(pod) && !c.isAppPodUnready(ctx, pod) {
		// skip
		return reconcile.Result{}, nil
	}
	if pod.DeletionTimestamp != nil {
		c.appIndexes.removeIndex(req.NamespacedName)
		klog.V(6).Infof("pod %s deleted", req.NamespacedName)
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
	indexes.addIndex(
		pod,
		func(p *corev1.Pod) metav1.ObjectMeta { return p.ObjectMeta },
		func(name types.NamespacedName) (*corev1.Pod, error) {
			var pod corev1.Pod
			err := c.cachedReader.Get(ctx, name, &pod)
			return &pod, err
		},
	)
	klog.V(6).Infof("pod %s created", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *PodController) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("pod", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	return ctr.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			pod := deleteEvent.Object.(*corev1.Pod)
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
				klog.V(6).Infof("pod %s%s deleted", pod.GetNamespace(), pod.GetName())
				return false
			}
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	})
}

func (c *PVController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pv := &corev1.PersistentVolume{}
	if err := c.cachedReader.Get(ctx, req.NamespacedName, pv); err != nil {
		if k8serrors.IsNotFound(err) {
			c.pvIndexes.removeIndex(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		klog.Errorf("get pv %s failed: %v", req.NamespacedName, err)
		return reconcile.Result{}, err
	}
	if pv.DeletionTimestamp != nil {
		klog.V(6).Infof("watch pv %s deleted", req.NamespacedName)
		c.pvIndexes.removeIndex(req.NamespacedName)
		if pv.Spec.ClaimRef != nil {
			pvcName := types.NamespacedName{
				Namespace: pv.Spec.ClaimRef.Namespace,
				Name:      pv.Spec.ClaimRef.Name,
			}
			c.pairLock.Lock()
			delete(c.pairs, pvcName)
			c.pairLock.Unlock()
			c.pvcIndexes.removeIndex(pvcName)
		}
		return reconcile.Result{}, nil
	}
	c.pvIndexes.addIndex(
		pv,
		func(p *corev1.PersistentVolume) metav1.ObjectMeta { return p.ObjectMeta },
		func(name types.NamespacedName) (*corev1.PersistentVolume, error) {
			var p corev1.PersistentVolume
			err := c.cachedReader.Get(ctx, name, &p)
			return &p, err
		},
	)
	if pv.Spec.ClaimRef != nil {
		pvcName := types.NamespacedName{
			Namespace: pv.Spec.ClaimRef.Namespace,
			Name:      pv.Spec.ClaimRef.Name,
		}
		c.pairLock.Lock()
		c.pairs[pvcName] = req.NamespacedName
		c.pairLock.Unlock()
		var pvc corev1.PersistentVolumeClaim
		if err := c.cachedReader.Get(ctx, pvcName, &pvc); err != nil {
			klog.Errorf("get pvc %s failed: %v", pvcName, err)
			return reconcile.Result{}, nil
		}
		c.pvcIndexes.addIndex(
			&pvc,
			func(p *corev1.PersistentVolumeClaim) metav1.ObjectMeta { return p.ObjectMeta },
			func(name types.NamespacedName) (*corev1.PersistentVolumeClaim, error) {
				var p corev1.PersistentVolumeClaim
				err := c.cachedReader.Get(ctx, name, &p)
				return &p, err
			},
		)
	}
	klog.V(6).Infof("pv %s created", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *PVController) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("pv", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}
	return ctr.Watch(&source.Kind{Type: &corev1.PersistentVolume{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			pv := event.Object.(*corev1.PersistentVolume)
			return pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			pv := deleteEvent.Object.(*corev1.PersistentVolume)
			return pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	})
}

// func (c *PVCController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
// 	pv := &corev1.PersistentVolume{}
// 	if err := c.cachedReader.Get(ctx, req.NamespacedName, pv); err != nil {
// 		klog.Errorf("get pv %s failed: %v", req.NamespacedName, err)
// 		return reconcile.Result{}, nil
// 	}
// 	if pv.DeletionTimestamp != nil {
// 		return reconcile.Result{}, nil
// 	}
// 	c.pvIndexes.addIndex(
// 		pv,
// 		func(p *corev1.PersistentVolume) metav1.ObjectMeta { return p.ObjectMeta },
// 		func(name types.NamespacedName) (*corev1.PersistentVolume, error) {
// 			var p corev1.PersistentVolume
// 			err := c.cachedReader.Get(ctx, name, &p)
// 			return &p, err
// 		},
// 	)
// 	if pv.Spec.ClaimRef != nil {
// 		pvcName := types.NamespacedName{
// 			Namespace: pv.Spec.ClaimRef.Namespace,
// 			Name:      pv.Spec.ClaimRef.Name,
// 		}
// 		c.pairLock.Lock()
// 		c.pairs[pvcName] = req.NamespacedName
// 		c.pairLock.Unlock()
// 	}
// 	klog.V(6).Infof("pv %s created", req.NamespacedName)
// 	return reconcile.Result{}, nil
// }

// func (c *PVCController) SetupWithManager(mgr manager.Manager) error {
// 	ctr, err := controller.New("pvc", mgr, controller.Options{Reconciler: c})
// 	if err != nil {
// 		return err
// 	}

// 	return ctr.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
// 		CreateFunc: func(event event.CreateEvent) bool {
// 			pvc := event.Object.(*corev1.PersistentVolumeClaim)

// 			klog.V(6).Infof("watch pvc %s/%s created", pvc.GetNamespace(), pvc.GetName())
// 			return true
// 		},
// 		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
// 			return false
// 		},
// 		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
// 			pvc := deleteEvent.Object.(*corev1.PersistentVolumeClaim)
// 			name := types.NamespacedName{
// 				Namespace: pvc.GetNamespace(),
// 				Name:      pvc.GetName(),
// 			}
// 			klog.V(6).Infof("watch pv %s deleted", name)
// 			c.pvIndexes.removeIndex(name)
// 			if pv.Spec.ClaimRef != nil {
// 				pvcName := types.NamespacedName{
// 					Namespace: pv.Spec.ClaimRef.Namespace,
// 					Name:      pv.Spec.ClaimRef.Name,
// 				}
// 				c.pairLock.Lock()
// 				delete(c.pairs, pvcName)
// 				c.pairLock.Unlock()
// 			}
// 			return false
// 		},
// 		GenericFunc: func(genericEvent event.GenericEvent) bool {
// 			return false
// 		},
// 	})
// }

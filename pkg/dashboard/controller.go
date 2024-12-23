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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

var mgrLog = klog.NewKlogr().WithName("manager")

func (api *API) StartManager(ctx context.Context, mgr manager.Manager) error {
	podCtr := PodController{api}
	pvCtr := PVController{api}
	pvcCtr := PVCController{api}
	jobCtr := JobController{api}
	secretCtr := SecretController{api}
	if err := podCtr.SetupWithManager(mgr); err != nil {
		return err
	}
	if err := pvCtr.SetupWithManager(mgr); err != nil {
		return err
	}
	if err := pvcCtr.SetupWithManager(mgr); err != nil {
		return err
	}
	if err := jobCtr.SetupWithManager(mgr); err != nil {
		return err
	}
	if err := secretCtr.SetupWithManager(mgr); err != nil {
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

type JobController struct {
	*API
}

type SecretController struct {
	*API
}

func (c *PodController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}
	if err := c.cachedReader.Get(ctx, req.NamespacedName, pod); err != nil {
		mgrLog.Error(err, "get pod failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	if !isSysPod(pod) && !isAppPod(pod) && !c.isAppPodShouldList(ctx, pod) {
		// skip
		return reconcile.Result{}, nil
	}
	if pod.DeletionTimestamp != nil {
		c.appIndexes.removeIndex(req.NamespacedName)
		if isCsiNode(pod) {
			c.csiNodeLock.Lock()
			delete(c.csiNodeIndex, pod.Spec.NodeName)
			c.csiNodeLock.Unlock()
		}
		mgrLog.V(1).Info("pod deleted", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	indexes := c.appIndexes
	if isSysPod(pod) {
		indexes = c.sysIndexes
		if isCsiNode(pod) && pod.Spec.NodeName != "" {
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
	mgrLog.V(1).Info("pod created", "namespacedName", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *PodController) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("pod", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}

	return ctr.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, &handler.TypedEnqueueRequestForObject[*corev1.Pod]{}, predicate.TypedFuncs[*corev1.Pod]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.Pod]) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.Pod]) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.Pod]) bool {
			pod := deleteEvent.Object
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
				if isCsiNode(pod) {
					c.csiNodeLock.Lock()
					delete(c.csiNodeIndex, pod.Spec.NodeName)
					c.csiNodeLock.Unlock()
				}
				mgrLog.V(1).Info("pod deleted", "namespace", pod.GetNamespace(), "name", pod.GetName())
				return false
			}
			return true
		},
	}))
}

func (c *PVController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pv := &corev1.PersistentVolume{}
	if err := c.cachedReader.Get(ctx, req.NamespacedName, pv); err != nil {
		if k8serrors.IsNotFound(err) {
			c.pvIndexes.removeIndex(req.NamespacedName)
			return reconcile.Result{}, nil
		}
		mgrLog.Error(err, "get pv failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, err
	}
	if pv.DeletionTimestamp != nil {
		mgrLog.V(1).Info("watch pv deleted", "namespacedName", req.NamespacedName)
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
			mgrLog.Error(err, "get pvc failed", "name", pvcName)
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
	mgrLog.V(1).Info("pv created", "namespacedName", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *PVController) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("pv", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}
	return ctr.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolume{}, &handler.TypedEnqueueRequestForObject[*corev1.PersistentVolume]{}, predicate.TypedFuncs[*corev1.PersistentVolume]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.PersistentVolume]) bool {
			pv := event.Object
			return pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.PersistentVolume]) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.PersistentVolume]) bool {
			pv := deleteEvent.Object
			return pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName
		},
	}))
}

func (c *PVCController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := c.cachedReader.Get(ctx, req.NamespacedName, pvc); err != nil {
		mgrLog.Error(err, "get pvc failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	if pvc.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}
	if pvc.Status.Phase == corev1.ClaimPending {
		// created
		c.pvcIndexes.addIndex(
			pvc,
			func(p *corev1.PersistentVolumeClaim) metav1.ObjectMeta { return p.ObjectMeta },
			func(name types.NamespacedName) (*corev1.PersistentVolumeClaim, error) {
				var p corev1.PersistentVolumeClaim
				err := c.cachedReader.Get(ctx, name, &p)
				return &p, err
			},
		)
		return reconcile.Result{}, nil
	}
	if pvc.Status.Phase == corev1.ClaimBound {
		// updated
		c.pairLock.RLock()
		p, ok := c.pairs[req.NamespacedName]
		c.pairLock.RUnlock()
		if ok && p.Name == pvc.Spec.VolumeName {
			return reconcile.Result{}, nil
		}
		pvName := types.NamespacedName{
			Name: pvc.Spec.VolumeName,
		}
		var pv corev1.PersistentVolume
		if err := c.cachedReader.Get(ctx, pvName, &pv); err != nil {
			mgrLog.Error(err, "get pv failed", "name", pvName)
			return reconcile.Result{}, err
		}
		c.pairLock.Lock()
		defer c.pairLock.Unlock()
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
			c.pairs[req.NamespacedName] = pvName
		} else {
			delete(c.pairs, req.NamespacedName)
			c.pvcIndexes.removeIndex(req.NamespacedName)
		}
	}
	return reconcile.Result{}, nil
}

func (c *PVCController) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("pvc", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	return ctr.Watch(source.Kind(mgr.GetCache(), &corev1.PersistentVolumeClaim{}, &handler.TypedEnqueueRequestForObject[*corev1.PersistentVolumeClaim]{}, predicate.TypedFuncs[*corev1.PersistentVolumeClaim]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.PersistentVolumeClaim]) bool {
			pvc := event.Object
			// bound pvc should be added by pv controller
			return pvc.Status.Phase == corev1.ClaimPending || pvc.Status.Phase == corev1.ClaimBound
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.PersistentVolumeClaim]) bool {
			oldPvc, newPvc := updateEvent.ObjectOld, updateEvent.ObjectNew
			if oldPvc.Status.Phase == corev1.ClaimBound && newPvc.Status.Phase != corev1.ClaimBound {
				// pvc unbound
				c.pairLock.Lock()
				delete(c.pairs, types.NamespacedName{Namespace: oldPvc.GetNamespace(), Name: oldPvc.GetName()})
				c.pairLock.Unlock()
				return false
			}
			if oldPvc.Status.Phase == corev1.ClaimPending && newPvc.Status.Phase == corev1.ClaimBound {
				// pvc bound
				return true
			}
			return false
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.PersistentVolumeClaim]) bool {
			pvc := deleteEvent.Object
			name := types.NamespacedName{
				Namespace: pvc.GetNamespace(),
				Name:      pvc.GetName(),
			}
			mgrLog.V(1).Info("watch pvc deleted", "name", name)
			c.pvcIndexes.removeIndex(name)
			c.pairLock.Lock()
			delete(c.pairs, name)
			c.pairLock.Unlock()
			return false
		},
	}))
}

func (c *JobController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	job := &batchv1.Job{}
	if err := c.cachedReader.Get(ctx, req.NamespacedName, job); err != nil {
		mgrLog.Error(err, "get job failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	if job.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}
	if isUpgradeJob(job) {
		c.jobsIndexes.addIndex(
			job,
			func(p *batchv1.Job) metav1.ObjectMeta { return p.ObjectMeta },
			func(name types.NamespacedName) (*batchv1.Job, error) {
				var j batchv1.Job
				err := c.cachedReader.Get(ctx, name, &j)
				return &j, err
			},
		)
	}
	mgrLog.V(1).Info("job created", "namespacedName", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *JobController) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("job", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	return ctr.Watch(source.Kind(mgr.GetCache(), &batchv1.Job{}, &handler.TypedEnqueueRequestForObject[*batchv1.Job]{}, predicate.TypedFuncs[*batchv1.Job]{
		CreateFunc: func(event event.TypedCreateEvent[*batchv1.Job]) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*batchv1.Job]) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*batchv1.Job]) bool {
			job := deleteEvent.Object
			indexes := c.jobsIndexes
			if isUpgradeJob(job) && indexes != nil {
				indexes.removeIndex(types.NamespacedName{
					Namespace: job.GetNamespace(),
					Name:      job.GetName(),
				})
				mgrLog.V(1).Info("job deleted", "namespace", job.GetNamespace(), "name", job.GetName())
				return false
			}
			return true
		},
	}))
}

func (c *SecretController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	secret := &corev1.Secret{}
	if err := c.cachedReader.Get(ctx, req.NamespacedName, secret); err != nil {
		mgrLog.Error(err, "get secret failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	if secret.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}
	if isJuiceSecret(secret) || isJuiceCustSecret(secret) {
		c.secretIndexes.addIndex(
			secret,
			func(p *corev1.Secret) metav1.ObjectMeta { return p.ObjectMeta },
			func(name types.NamespacedName) (*corev1.Secret, error) {
				var s corev1.Secret
				err := c.cachedReader.Get(ctx, name, &s)
				return &s, err
			},
		)
	}
	mgrLog.V(1).Info("secret created", "namespacedName", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *SecretController) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("secret", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	return ctr.Watch(source.Kind(mgr.GetCache(), &corev1.Secret{}, &handler.TypedEnqueueRequestForObject[*corev1.Secret]{}, predicate.TypedFuncs[*corev1.Secret]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.Secret]) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.Secret]) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.Secret]) bool {
			secret := deleteEvent.Object
			indexes := c.secretIndexes
			if indexes != nil && (isJuiceSecret(secret) || isJuiceCustSecret(secret)) {
				indexes.removeIndex(types.NamespacedName{
					Namespace: secret.GetNamespace(),
					Name:      secret.GetName(),
				})
				mgrLog.V(1).Info("secret deleted", "namespace", secret.GetNamespace(), "name", secret.GetName())
				return false
			}
			return true
		},
	}))
}

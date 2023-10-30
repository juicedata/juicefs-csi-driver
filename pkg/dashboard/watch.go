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
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func (api *API) watchAppPod(ctx context.Context) {
	watcher, err := api.watchPod(ctx)
	if err != nil {
		klog.V(0).Infof("can't watch pods: %v", err)
		return
	}
	for e := range watcher.ResultChan() {
		go func(event watch.Event) {
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				klog.V(0).Infof("unknown type: %v", event.Object)
				return
			}

			// check if pod use JuiceFS Volume
			var used bool
			if pod.Labels != nil {
				// mount pod mode
				if _, ok := pod.Labels[config.UniqueId]; ok {
					used = true
				}
				// sidecar mode
				if _, ok := pod.Labels[config.InjectSidecarDone]; ok {
					used = true
				}
			}
			if !used {
				// api.pvcs contain all pending pvc & juicefs pvc, we should get pod if pvc pending.
				for _, volume := range pod.Spec.Volumes {
					if volume.PersistentVolumeClaim != nil {
						api.pvsLock.RLock()
						if _, ok := api.pvcs[types.NamespacedName{Name: volume.PersistentVolumeClaim.ClaimName, Namespace: pod.Namespace}]; ok {
							used = true
						}
						api.pvsLock.RUnlock()
					}
				}
				if !used {
					return
				}
			}
			name := types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}
			switch event.Type {
			case watch.Added:
				api.indexAppPod(name, pod)
			case watch.Modified, watch.Error:
				api.updateAppPod(name, pod)
			case watch.Deleted:
				api.removeAppPod(name)
			}
		}(e)
	}
}

func (api *API) indexAppPod(name types.NamespacedName, pod *corev1.Pod) {
	api.appPodsLock.Lock()
	defer api.appPodsLock.Unlock()
	if _, ok := api.appPods[name]; ok {
		return
	}
	api.appPods[name] = pod
	api.appIndexes.addIndex(name, pod, api.appPods)
}

func (api *API) updateAppPod(name types.NamespacedName, pod *corev1.Pod) {
	api.appPodsLock.Lock()
	defer api.appPodsLock.Unlock()
	api.appPods[name] = pod
}

func (api *API) removeAppPod(name types.NamespacedName) {
	api.appPodsLock.Lock()
	defer api.appPodsLock.Unlock()
	delete(api.appPods, name)
	api.appIndexes.removeIndex(name)
}

func (api *API) watchRelatedPV(ctx context.Context) {
	watcher, err := api.watchPV(ctx)
	if err != nil {
		klog.V(0).Infof("fail to watch pv: %v", err)
		return
	}
	for e := range watcher.ResultChan() {
		go func(event watch.Event) {
			pv, ok := event.Object.(*corev1.PersistentVolume)
			if !ok {
				klog.V(0).Infof("unknown type: %v", event.Object)
				return
			}

			// if PV is JuiceFS PV
			if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != config.DriverName {
				return
			}

			// get pvc if bounded
			var pvc *corev1.PersistentVolumeClaim
			if pv.Spec.ClaimRef != nil {
				pvc, err = api.k8sClient.GetPersistentVolumeClaim(ctx, pv.Spec.ClaimRef.Name, pv.Spec.ClaimRef.Namespace)
				if err != nil {
					klog.V(0).Infof("get pvc error %v", err)
				}
			}
			func() {
				api.pvsLock.Lock()
				defer api.pvsLock.Unlock()
				name := api.sysNamespaced(pv.Name)
				switch event.Type {
				case watch.Added:
					if _, ok := api.pvs[name]; ok {
						return
					}
					api.pvs[name] = pv
					api.pvIndexes.addIndex(name, pv, api.pvs)
					if pvc != nil {
						pvcName := types.NamespacedName{
							Namespace: pvc.Namespace,
							Name:      pvc.Name,
						}
						if _, ok := api.pvcs[pvcName]; ok {
							return
						}
						api.pairs[pvcName] = name
						api.pvcs[pvcName] = pvc
						api.pvcIndexes.addIndex(pvcName, pvc, api.pvcs)
					}
				case watch.Modified, watch.Error:
					api.pvs[name] = pv
				case watch.Deleted:
					delete(api.pvs, name)
					api.pvIndexes.removeIndex(name)
					if pvc != nil {
						pvcName := types.NamespacedName{
							Namespace: pvc.Namespace,
							Name:      pvc.Name,
						}
						delete(api.pairs, pvcName)
						delete(api.pvcs, pvcName)
					}
				}
			}()
		}(e)
	}
}

func (api *API) watchRelatedPVC(ctx context.Context) {
	watcher, err := api.watchPVC(ctx)
	if err != nil {
		klog.V(0).Infof("fail to watch pvc: %v", err)
		return
	}
	for e := range watcher.ResultChan() {
		go func(event watch.Event) {
			api.pvsLock.Lock()
			defer api.pvsLock.Unlock()

			pvc, ok := event.Object.(*corev1.PersistentVolumeClaim)
			if !ok {
				klog.V(0).Infof("unknown type: %v", event.Object)
				return
			}

			pvcName := types.NamespacedName{
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
			}
			// if PVC bound and is not JuiceFS PVC, return
			if pvc.Status.Phase == corev1.ClaimBound {
				if _, ok = api.pvcs[pvcName]; !ok {
					return
				}
			}

			switch event.Type {
			case watch.Added:
				if _, ok := api.pvcs[pvcName]; ok {
					return
				}
				api.pvcs[pvcName] = pvc
				api.pvcIndexes.addIndex(pvcName, pvc, api.pvcs)
			case watch.Modified, watch.Error:
				api.pvcs[pvcName] = pvc
			case watch.Deleted:
				delete(api.pvcs, pvcName)
				delete(api.pairs, pvcName)
				api.pvcIndexes.removeIndex(pvcName)
			}
		}(e)
	}
}

func (api *API) watchNodes(ctx context.Context) {
	watcher, err := api.watchNode(ctx)
	if err != nil {
		klog.V(0).Infof("fail to watch node: %v", err)
		return
	}
	for e := range watcher.ResultChan() {
		go func(event watch.Event) {
			api.nodesLock.Lock()
			defer api.nodesLock.Unlock()

			node, ok := event.Object.(*corev1.Node)
			if !ok {
				klog.V(0).Infof("unknown type: %v", event.Object)
				return
			}

			switch event.Type {
			case watch.Added, watch.Modified, watch.Error:
				api.nodes[node.Name] = node
			case watch.Deleted:
				delete(api.nodes, node.Name)
			}
		}(e)
	}
}

func (api *API) watchPodByLabels(ctx context.Context, labels map[string]string) (watch.Interface, error) {
	return api.watchPodByLabelSelector(ctx, &v1.LabelSelector{MatchLabels: labels})
}

func (api *API) watchPodByLabelSelector(ctx context.Context, selector *v1.LabelSelector) (watch.Interface, error) {
	s, err := v1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, errors.Wrapf(err, "can't convert label selector %v", selector)
	}
	watcher, err := api.k8sClient.CoreV1().Pods("").Watch(ctx, v1.ListOptions{
		LabelSelector: s.String(),
		Watch:         true,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "can't watch pods by %s", s.String())
	}
	return watcher, nil
}

func (api *API) watchPod(ctx context.Context) (watch.Interface, error) {
	watcher, err := api.k8sClient.CoreV1().Pods("").Watch(ctx, v1.ListOptions{
		Watch: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't watch pods")
	}
	return watcher, nil
}

func (api *API) watchPV(ctx context.Context) (watch.Interface, error) {
	watcher, err := api.k8sClient.CoreV1().PersistentVolumes().Watch(ctx, v1.ListOptions{
		Watch: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't watch pods")
	}
	return watcher, nil
}

func (api *API) watchPVC(ctx context.Context) (watch.Interface, error) {
	watcher, err := api.k8sClient.CoreV1().PersistentVolumeClaims("").Watch(ctx, v1.ListOptions{
		Watch: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't watch pods")
	}
	return watcher, nil
}

func (api *API) watchNode(ctx context.Context) (watch.Interface, error) {
	watcher, err := api.k8sClient.CoreV1().Nodes().Watch(ctx, v1.ListOptions{
		Watch: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't watch pods")
	}
	return watcher, nil
}

func (api *API) watchComponents(ctx context.Context) {
	mountPodWatcher, err := api.watchPodByLabels(ctx, map[string]string{"app.kubernetes.io/name": "juicefs-mount"})
	if err != nil {
		klog.V(0).Infof("fail to watch mount pod: %v", err)
		return
	}
	csiNodeWatcher, err := api.watchPodByLabels(ctx, map[string]string{
		"app.kubernetes.io/name": "juicefs-csi-driver",
		"app":                    "juicefs-csi-node",
	})
	if err != nil {
		klog.V(0).Infof("fail to watch csi-nodes: %v", err)
		return
	}
	csiControllerWatcher, err := api.watchPodByLabels(ctx, map[string]string{
		"app.kubernetes.io/name": "juicefs-csi-driver",
		"app":                    "juicefs-csi-controller",
	})
	if err != nil {
		klog.V(0).Infof("fail to watch controllers: %v", err)
		return
	}
	go api.processSysEvents(ctx, mountPodWatcher.ResultChan(), api.mountPods, nil)
	go api.processSysEvents(ctx, csiControllerWatcher.ResultChan(), api.controllers, nil)
	go api.processSysEvents(ctx, csiNodeWatcher.ResultChan(), api.csiNodes, func(eventType watch.EventType, pod *corev1.Pod) {
		switch eventType {
		case watch.Added, watch.Modified, watch.Error:
			api.csiNodeIndex[pod.Spec.NodeName] = pod
		case watch.Deleted:
			delete(api.csiNodeIndex, pod.Spec.NodeName)
		}
	})
}

func (api *API) processSysEvents(ctx context.Context, events <-chan watch.Event, table map[types.NamespacedName]*corev1.Pod, nodeIndex func(watch.EventType, *corev1.Pod)) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			if event.Object == nil {
				klog.V(0).Infof("get nil event: %s", event.Type)
				continue
			}
			func() {
				api.componentsLock.Lock()
				defer api.componentsLock.Unlock()
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					klog.V(0).Infof("unknown type: %v", event.Object)
					return
				}
				if nodeIndex != nil {
					nodeIndex(event.Type, pod)
				}
				spacedName := api.sysNamespaced(pod.Name)
				switch event.Type {
				case watch.Added:
					if _, ok := table[spacedName]; ok {
						return
					}
					table[spacedName] = pod
					api.sysIndexes.addIndex(spacedName, pod, api.mountPods, api.csiNodes, api.controllers)
				case watch.Modified, watch.Error:
					table[spacedName] = pod
				case watch.Deleted:
					delete(table, api.sysNamespaced(pod.Name))
					api.sysIndexes.removeIndex(spacedName)
				}
			}()
		}
	}
}

func (api *API) watchPodEvents(ctx context.Context) {
	watcher, err := api.k8sClient.CoreV1().Events("").Watch(ctx, v1.ListOptions{
		TypeMeta: v1.TypeMeta{Kind: "Pod"},
		Watch:    true,
	})
	if err != nil {
		klog.V(0).Infof("can't watch event of pods: %v", err)
		return
	}
	for e := range watcher.ResultChan() {
		go func(we watch.Event) {
			api.eventsLock.Lock()
			event, ok := we.Object.(*corev1.Event)
			if !ok {
				api.eventsLock.Unlock()
				klog.V(0).Infof("unknown type: %v", we.Object)
				return
			}
			objName := types.NamespacedName{
				Namespace: event.InvolvedObject.Namespace,
				Name:      event.InvolvedObject.Name,
			}

			switch we.Type {
			case watch.Added:
				if api.events[objName] == nil {
					api.events[objName] = make(map[string]*corev1.Event, 1)
				}
				api.events[objName][string(event.UID)] = event
			case watch.Deleted:
				delete(api.events[objName], string(event.UID))
			}
			api.eventsLock.Unlock()
		}(e)
	}
}

func (api *API) watchPVEvents(ctx context.Context) {
	watcher, err := api.k8sClient.CoreV1().Events("").Watch(ctx, v1.ListOptions{
		TypeMeta: v1.TypeMeta{Kind: "PersistentVolume"},
		Watch:    true,
	})
	if err != nil {
		klog.V(0).Infof("can't watch event of PV: %v", err)
		return
	}
	for e := range watcher.ResultChan() {
		go func(we watch.Event) {
			api.eventsLock.Lock()
			event, ok := we.Object.(*corev1.Event)
			if !ok {
				api.eventsLock.Unlock()
				klog.V(0).Infof("unknown type: %v", we.Object)
				return
			}
			objName := types.NamespacedName{
				Namespace: event.InvolvedObject.Namespace,
				Name:      event.InvolvedObject.Name,
			}

			switch we.Type {
			case watch.Added:
				if api.events[objName] == nil {
					api.events[objName] = make(map[string]*corev1.Event, 1)
				}
				api.events[objName][string(event.UID)] = event
			case watch.Deleted:
				delete(api.events[objName], string(event.UID))
			}
			api.eventsLock.Unlock()
		}(e)
	}
}

func (api *API) watchPVCEvents(ctx context.Context) {
	watcher, err := api.k8sClient.CoreV1().Events("").Watch(ctx, v1.ListOptions{
		TypeMeta: v1.TypeMeta{Kind: "PersistentVolumeClaim"},
		Watch:    true,
	})
	if err != nil {
		klog.V(0).Infof("can't watch event of PVC: %v", err)
		return
	}
	for e := range watcher.ResultChan() {
		go func(we watch.Event) {
			api.eventsLock.Lock()
			event, ok := we.Object.(*corev1.Event)
			if !ok {
				api.eventsLock.Unlock()
				klog.V(0).Infof("unknown type: %v", we.Object)
				return
			}
			objName := types.NamespacedName{
				Namespace: event.InvolvedObject.Namespace,
				Name:      event.InvolvedObject.Name,
			}

			switch we.Type {
			case watch.Added:
				if api.events[objName] == nil {
					api.events[objName] = make(map[string]*corev1.Event, 1)
				}
				api.events[objName][string(event.UID)] = event
			case watch.Deleted:
				delete(api.events[objName], string(event.UID))
			}
			api.eventsLock.Unlock()
		}(e)
	}
}

func (api *API) cleanupEvents(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(10 * time.Second)
			api.eventsLock.Lock()
			for name := range api.events {
				if !api.isComponents(name.Name) && !api.isApp(name) && !api.isPV(name) && !api.isPVC(name) {
					delete(api.events, name)
					klog.V(3).Infof("delete all events of pod %s\n", name)
				}
			}
			api.eventsLock.Unlock()
		}
	}
}

func (api *API) isComponents(name string) bool {
	_, exist := api.getComponentPod(api.sysNamespaced(name))
	return exist
}

func (api *API) isApp(name types.NamespacedName) bool {
	pod := api.getAppPod(name)
	return pod != nil
}

func (api *API) isPV(name types.NamespacedName) bool {
	pv := api.getPV(name.Name)
	return pv != nil
}

func (api *API) isPVC(name types.NamespacedName) bool {
	pvc := api.getPVC(name.Namespace, name.Name)
	return pvc != nil
}

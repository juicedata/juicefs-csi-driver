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
	"log"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func (api *API) watchAppPod(ctx context.Context) {
	watcher, err := api.watchPod(ctx)
	if err != nil {
		log.Fatalf("%v", err)
	}
	for event := range watcher.ResultChan() {
		func() {
			api.appPodsLock.Lock()
			defer api.appPodsLock.Unlock()

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				log.Printf("unknown type: %v", event.Object)
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
				used, _, err := util.GetVolumes(ctx, api.k8sClient, pod)
				if err != nil {
					log.Printf("get volumes error %v", err)
					return
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
			case watch.Added, watch.Modified, watch.Error:
				api.appPods[name] = pod
			case watch.Deleted:
				delete(api.appPods, name)
			}
		}()
	}
}

func (api *API) watchRelatedPV(ctx context.Context) {
	watcher, err := api.watchPV(ctx)
	if err != nil {
		log.Fatalf("%v", err)
	}
	for event := range watcher.ResultChan() {
		func() {
			api.pvsLock.Lock()
			defer api.pvsLock.Unlock()

			pv, ok := event.Object.(*corev1.PersistentVolume)
			if !ok {
				log.Printf("unknown type: %v", event.Object)
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
					log.Printf("get pvc error %v", err)
				}
			}
			switch event.Type {
			case watch.Added, watch.Modified, watch.Error:
				api.pvs[pv.Name] = pv
				if pvc != nil {
					pvcName := types.NamespacedName{
						Namespace: pvc.Namespace,
						Name:      pvc.Name,
					}
					api.pairs[pvcName] = pv.Name
					api.pvcs[pvcName] = pvc
				}
			case watch.Deleted:
				delete(api.pvs, pv.Name)
				if pvc != nil {
					pvcName := types.NamespacedName{
						Namespace: pvc.Namespace,
						Name:      pvc.Name,
					}
					delete(api.pairs, pvcName)
				}
			}
		}()
	}
}

func (api *API) watchRelatedPVC(ctx context.Context) {
	watcher, err := api.watchPVC(ctx)
	if err != nil {
		log.Fatalf("%v", err)
	}
	for event := range watcher.ResultChan() {
		func() {
			api.pvsLock.Lock()
			defer api.pvsLock.Unlock()

			pvc, ok := event.Object.(*corev1.PersistentVolumeClaim)
			if !ok {
				log.Printf("unknown type: %v", event.Object)
				return
			}

			pvcName := types.NamespacedName{
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
			}
			// if PVC is not JuiceFS PVC, return
			_, ok = api.pvcs[pvcName]
			if !ok {
				return
			}

			switch event.Type {
			case watch.Added, watch.Modified, watch.Error:
				api.pvcs[pvcName] = pvc
			case watch.Deleted:
				delete(api.pvcs, pvcName)
				delete(api.pairs, pvcName)
			}
		}()
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

func (api *API) watchComponents(ctx context.Context) {
	mountPodWatcher, err := api.watchPodByLabels(ctx, map[string]string{"app.kubernetes.io/name": "juicefs-mount"})
	if err != nil {
		log.Fatalf("%v", err)
	}
	csiNodeWatcher, err := api.watchPodByLabels(ctx, map[string]string{
		"app.kubernetes.io/name": "juicefs-csi-driver",
		"app":                    "juicefs-csi-node",
	})
	if err != nil {
		log.Fatalf("%v", err)
	}
	csiControllerWatcher, err := api.watchPodByLabels(ctx, map[string]string{
		"app.kubernetes.io/name": "juicefs-csi-driver",
		"app":                    "juicefs-csi-controller",
	})
	if err != nil {
		log.Fatalf("%v", err)
	}
	watchers := []watch.Interface{mountPodWatcher, csiNodeWatcher, csiControllerWatcher}
	tables := []map[string]*corev1.Pod{api.mountPods, api.csiNodes, api.controllers}
	indexes := []func(watch.EventType, *corev1.Pod){
		nil,
		func(eventType watch.EventType, pod *corev1.Pod) {
			switch eventType {
			case watch.Added, watch.Modified, watch.Error:
				api.nodeindex[pod.Spec.NodeName] = pod
			case watch.Deleted:
				delete(api.nodeindex, pod.Spec.NodeName)
			}
		},
		nil,
	}
	for i := range watchers {
		go func(watcher watch.Interface, table map[string]*corev1.Pod, index func(watch.EventType, *corev1.Pod)) {
			for event := range watcher.ResultChan() {
				api.componentsLock.Lock()
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					api.componentsLock.Unlock()
					log.Printf("unknown type: %v", event.Object)
					continue
				}
				if index != nil {
					index(event.Type, pod)
				}
				switch event.Type {
				case watch.Added, watch.Modified, watch.Error:
					table[pod.Name] = pod
				case watch.Deleted:
					delete(table, pod.Name)
				}
				api.componentsLock.Unlock()
			}
		}(watchers[i], tables[i], indexes[i])
	}
}

func (api *API) watchPodEvents(ctx context.Context) {
	watcher, err := api.k8sClient.CoreV1().Events("").Watch(ctx, v1.ListOptions{
		TypeMeta: v1.TypeMeta{Kind: "Pod"},
		Watch:    true,
	})
	if err != nil {
		log.Fatalf("can't watch event of pods: %v", err)
	}
	for e := range watcher.ResultChan() {
		api.eventsLock.Lock()
		event, ok := e.Object.(*corev1.Event)
		if !ok {
			api.eventsLock.Unlock()
			log.Printf("unknown type: %v", e.Object)
			continue
		}
		objName := types.NamespacedName{
			Namespace: event.InvolvedObject.Namespace,
			Name:      event.InvolvedObject.Name,
		}

		switch e.Type {
		case watch.Added:
			if api.events[objName] == nil {
				api.events[objName] = make(map[string]*corev1.Event, 1)
			}
			api.events[objName][string(event.UID)] = event
		case watch.Deleted:
			delete(api.events[objName], string(event.UID))
		}
		api.eventsLock.Unlock()
	}
}

func (api *API) watchPVEvents(ctx context.Context) {
	watcher, err := api.k8sClient.CoreV1().Events("").Watch(ctx, v1.ListOptions{
		TypeMeta: v1.TypeMeta{Kind: "PersistentVolume"},
		Watch:    true,
	})
	if err != nil {
		log.Fatalf("can't watch event of PV: %v", err)
	}
	for e := range watcher.ResultChan() {
		api.eventsLock.Lock()
		event, ok := e.Object.(*corev1.Event)
		if !ok {
			api.eventsLock.Unlock()
			log.Printf("unknown type: %v", e.Object)
			continue
		}
		objName := types.NamespacedName{
			Namespace: event.InvolvedObject.Namespace,
			Name:      event.InvolvedObject.Name,
		}

		switch e.Type {
		case watch.Added:
			if api.events[objName] == nil {
				api.events[objName] = make(map[string]*corev1.Event, 1)
			}
			api.events[objName][string(event.UID)] = event
		case watch.Deleted:
			delete(api.events[objName], string(event.UID))
		}
		api.eventsLock.Unlock()
	}
}

func (api *API) watchPVCEvents(ctx context.Context) {
	watcher, err := api.k8sClient.CoreV1().Events("").Watch(ctx, v1.ListOptions{
		TypeMeta: v1.TypeMeta{Kind: "PersistentVolumeClaim"},
		Watch:    true,
	})
	if err != nil {
		log.Fatalf("can't watch event of PVC: %v", err)
	}
	for e := range watcher.ResultChan() {
		api.eventsLock.Lock()
		event, ok := e.Object.(*corev1.Event)
		if !ok {
			api.eventsLock.Unlock()
			log.Printf("unknown type: %v", e.Object)
			continue
		}
		objName := types.NamespacedName{
			Namespace: event.InvolvedObject.Namespace,
			Name:      event.InvolvedObject.Name,
		}

		switch e.Type {
		case watch.Added:
			if api.events[objName] == nil {
				api.events[objName] = make(map[string]*corev1.Event, 1)
			}
			api.events[objName][string(event.UID)] = event
		case watch.Deleted:
			delete(api.events[objName], string(event.UID))
		}
		api.eventsLock.Unlock()
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
					log.Printf("delete all events of pod %s\n", name)
				}
			}
			api.eventsLock.Unlock()
		}
	}
}

func (api *API) isComponents(name string) bool {
	_, exist := api.getComponentPod(name)
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

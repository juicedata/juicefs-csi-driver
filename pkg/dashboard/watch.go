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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

func (api *API) watchAppPod(ctx context.Context) {
	labelSelector := &v1.LabelSelector{
		MatchExpressions: []v1.LabelSelectorRequirement{{Key: config.UniqueId, Operator: v1.LabelSelectorOpExists}},
	}
	watcher, err := api.watchPodByLabelSelector(ctx, labelSelector)
	if err != nil {
		log.Fatalf("%v", err)
	}
	for event := range watcher.ResultChan() {
		api.appPodsLock.Lock()
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			api.appPodsLock.Unlock()
			log.Printf("unknown type: %v", event.Object)
			continue
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
		api.appPodsLock.Unlock()
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
	for i := range watchers {
		go func(watcher watch.Interface, table map[string]*corev1.Pod) {
			for event := range watcher.ResultChan() {
				api.componentsLock.Lock()
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					api.componentsLock.Unlock()
					log.Printf("unknown type: %v", event.Object)
					continue
				}
				switch event.Type {
				case watch.Added, watch.Modified, watch.Error:
					table[pod.Name] = pod
				case watch.Deleted:
					delete(table, pod.Name)
				}
				api.componentsLock.Unlock()
			}
		}(watchers[i], tables[i])
	}
}

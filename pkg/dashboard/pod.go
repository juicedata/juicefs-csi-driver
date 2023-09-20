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
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type API struct {
	sysNamespace string
	k8sClient    *k8sclient.K8sClient

	componentsLock sync.RWMutex
	mountPods      map[string]*corev1.Pod
	csiNodes       map[string]*corev1.Pod
	controllers    map[string]*corev1.Pod

	appPodsLock sync.RWMutex
	appPods     map[types.NamespacedName]*corev1.Pod

	eventsLock sync.RWMutex
	events     map[string]map[string]*corev1.Event
}

func NewAPI(ctx context.Context, sysNamespace string, k8sClient *k8sclient.K8sClient) *API {
	api := &API{
		sysNamespace: sysNamespace,
		k8sClient:    k8sClient,
		mountPods:    make(map[string]*corev1.Pod),
		csiNodes:     make(map[string]*corev1.Pod),
		controllers:  make(map[string]*corev1.Pod),
		appPods:      make(map[types.NamespacedName]*corev1.Pod),
		events:       make(map[string]map[string]*corev1.Event),
	}
	go api.watchComponents(ctx)
	go api.watchAppPod(ctx)
	go api.watchPodEvents(ctx)
	go api.cleanupPodEvents(ctx)
	return api
}

func (api *API) Handle(group *gin.RouterGroup) {
	group.GET("/pods", api.listAppPod())
	group.GET("/mountpods", api.listMountPod())
	group.GET("/csi-nodes", api.listCSINodePod())
	group.GET("/controllers", api.listCSIControllerPod())
	podGroup := group.Group("/pod/:namespace/:name", api.getPodMiddileware())
	podGroup.GET("/", api.getPodHandler())
	podGroup.GET("/events", api.getPodEvents())
	podGroup.GET("/logs/:container", api.getPodLogs())
	podGroup.GET("/pvs", api.listPodPVsHandler())
	podGroup.GET("/mountpods", api.listMountPods())
}

func (api *API) listAppPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		pods := make([]*corev1.Pod, 0, len(api.appPods))
		api.appPodsLock.RLock()
		for _, pod := range api.appPods {
			pods = append(pods, pod)
		}
		api.appPodsLock.RUnlock()
		c.IndentedJSON(200, pods)
	}
}

func (api *API) listMountPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		pods := make([]*corev1.Pod, 0, len(api.mountPods))
		api.componentsLock.RLock()
		for _, pod := range api.mountPods {
			pods = append(pods, pod)
		}
		api.componentsLock.RUnlock()
		c.IndentedJSON(200, pods)
	}
}

func (api *API) listCSINodePod() gin.HandlerFunc {
	return func(c *gin.Context) {
		pods := make([]*corev1.Pod, 0, len(api.csiNodes))
		api.componentsLock.RLock()
		for _, pod := range api.csiNodes {
			pods = append(pods, pod)
		}
		api.componentsLock.RUnlock()
		c.IndentedJSON(200, pods)
	}
}

func (api *API) listCSIControllerPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		pods := make([]*corev1.Pod, 0, len(api.controllers))
		api.componentsLock.RLock()
		for _, pod := range api.controllers {
			pods = append(pods, pod)
		}
		api.componentsLock.RUnlock()
		c.IndentedJSON(200, pods)
	}
}

func (api *API) getComponentPod(name string) (*corev1.Pod, bool) {
	var pod *corev1.Pod
	var exist bool
	api.componentsLock.RLock()
	pod, exist = api.mountPods[name]
	if !exist {
		pod, exist = api.csiNodes[name]
	}
	if !exist {
		pod, exist = api.controllers[name]
	}
	api.componentsLock.RUnlock()
	return pod, exist
}

func (api *API) getAppPod(name types.NamespacedName) *corev1.Pod {
	api.appPodsLock.RLock()
	defer api.appPodsLock.RUnlock()
	return api.appPods[name]
}

func (api *API) getPodMiddileware() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		pod, exist := api.getComponentPod(name)
		if !exist {
			pod = api.getAppPod(types.NamespacedName{Namespace: namespace, Name: name})
		}
		if pod == nil {
			c.AbortWithStatus(404)
			return
		}
		c.Set("pod", pod)
	}
}

func (api *API) getPodHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pod, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		c.IndentedJSON(200, pod)
	}
}

func (api *API) getPodEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		pod, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		api.eventsLock.RLock()
		events := api.events[string(pod.(*corev1.Pod).Name)]
		list := make([]*corev1.Event, 0, len(events))
		for _, e := range events {
			list = append(list, e)
		}
		api.eventsLock.RUnlock()
		c.IndentedJSON(200, list)
	}
}

func (api *API) getPodLogs() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		container := c.Param("container")
		var existContainer bool
		for _, c := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
			if c.Name == container {
				existContainer = true
				break
			}
		}
		if !existContainer {
			c.String(404, "container %s not found", container)
			return
		}

		logs, err := api.k8sClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: container,
		}).DoRaw(c)
		if err != nil {
			c.String(500, "get pod logs of container %s: %v", container, err)
			return
		}
		c.String(200, string(logs))
	}
}

func (api *API) listPodPVsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		pvs, err := api.listPVsOfPod(c, pod)
		if err != nil {
			c.String(500, "list juicefs pvs: %v", err)
			return
		}
		c.IndentedJSON(200, pvs)
	}
}

func (api *API) listMountPods() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		pvs, err := api.listPVsOfPod(c, pod)
		if err != nil {
			c.String(500, "list juicefs pvs: %v", err)
			return
		}
		mountPods := make(map[string]*corev1.Pod)
		for pvc, pv := range pvs {
			key := fmt.Sprintf("%s-%s", config.JuiceFSMountPod, pv.Name)
			mountPodName, ok := pod.Annotations[key]
			if !ok {
				log.Printf("can't find mount pod name by annotation `%s`\n", key)
				continue
			}
			pair := strings.SplitN(mountPodName, string(types.Separator), 2)
			if len(pair) != 2 {
				log.Printf("invalid mount pod name %s\n", mountPodName)
				continue
			}
			api.componentsLock.RLock()
			mountPod, exist := api.mountPods[pair[1]]
			api.componentsLock.RUnlock()
			if !exist {
				log.Printf("mount pod %s not found\n", mountPodName)
				continue
			}
			mountPods[pvc] = mountPod
		}
		c.IndentedJSON(200, mountPods)
	}
}

func (api *API) watchPodEvents(ctx context.Context) {
	watcher, err := api.k8sClient.CoreV1().Events(api.sysNamespace).Watch(ctx, v1.ListOptions{
		TypeMeta: v1.TypeMeta{Kind: "Pod"},
		Watch:    true,
	})
	if err != nil {
		log.Fatalf("can't watch event of pods in %s: %v", api.sysNamespace, err)
	}
	for e := range watcher.ResultChan() {
		api.eventsLock.Lock()
		event, ok := e.Object.(*corev1.Event)
		if !ok {
			api.eventsLock.Unlock()
			log.Printf("unknown type: %v", e.Object)
			continue
		}
		objName := event.InvolvedObject.Name
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

func (api *API) cleanupPodEvents(ctx context.Context) {
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
				if !api.isComponents(name) {
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

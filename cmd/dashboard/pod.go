package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type podApi struct {
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

func newPodApi(ctx context.Context, sysNamespace string, k8sClient *k8sclient.K8sClient) *podApi {
	api := &podApi{
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

func (api *podApi) getComponentPod(name string) (*corev1.Pod, bool) {
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

func (api *podApi) getAppPod(name types.NamespacedName) *corev1.Pod {
	api.appPodsLock.RLock()
	defer api.appPodsLock.RUnlock()
	return api.appPods[name]
}

func (api *podApi) getPodMiddileware() gin.HandlerFunc {
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

func (api *podApi) getPodHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		pod, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		c.IndentedJSON(200, pod)
	}
}

func (api *podApi) getPodEvents() gin.HandlerFunc {
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

func (api *podApi) getPodLogs() gin.HandlerFunc {
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

func (api *podApi) watchPodEvents(ctx context.Context) {
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

func (api *podApi) cleanupPodEvents(ctx context.Context) {
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

func (api *podApi) isComponents(name string) bool {
	_, exist := api.getComponentPod(name)
	return exist
}

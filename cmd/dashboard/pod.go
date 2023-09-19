package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
)

type podApi struct {
	sysNamespace string
	k8sClient    *k8sclient.K8sClient

	componentsLock sync.RWMutex
	mountPods      map[string]*corev1.Pod
	csiNodes       map[string]*corev1.Pod
	controllers    map[string]*corev1.Pod

	appPodsLock sync.RWMutex
	appPods     map[string]*corev1.Pod

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
		appPods:      make(map[string]*corev1.Pod),
		events:       make(map[string]map[string]*corev1.Event),
	}
	go api.watchComponents(ctx)
	go api.watchAppPod(ctx)
	go api.watchPodEvents(ctx)
	go api.cleanupPodEvents(ctx)
	return api
}

func (api *podApi) getPodMiddileware() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		pod, err := api.k8sClient.GetPod(c, name, namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				c.AbortWithStatus(404)
			} else {
				c.AbortWithError(500, errors.Wrap(err, "get pod"))
			}
			return
		}
		if !isPermitted(pod) {
			klog.V(4).Infof("pod %s/%s is not permitted: %v", namespace, name, pod.Labels)
			c.AbortWithStatus(403)
			return
		}
		c.Set("pod", pod)
	}
}

func (api *podApi) getPod() gin.HandlerFunc {
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

func isPermitted(pod *corev1.Pod) bool {
	_, existUniqueId := pod.Labels[config.UniqueId]
	return existUniqueId ||
		pod.Labels["app.kubernetes.io/name"] == "juicefs-mount" ||
		pod.Labels["app.kubernetes.io/name"] == "juicefs-csi-driver"
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
	var ok bool
	api.componentsLock.RLock()
	_, ok = api.mountPods[name]
	if !ok {
		_, ok = api.csiNodes[name]
	}
	if !ok {
		_, ok = api.controllers[name]
	}
	api.componentsLock.RUnlock()
	return ok
}

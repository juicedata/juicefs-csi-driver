package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func (api *dashboardApi) listAppPod() gin.HandlerFunc {
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

func (api *dashboardApi) listPodByLabels(labels map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		selector := &v1.LabelSelector{
			MatchLabels: labels,
		}
		pods, err := api.k8sClient.ListPod(c, api.sysNamespace, selector, nil)
		if err != nil {
			c.String(500, "list pod: %v", err)
			return
		}
		c.IndentedJSON(200, pods)
	}
}

func (api *dashboardApi) listMountPod() gin.HandlerFunc {
	return api.listPodByLabels(map[string]string{"app.kubernetes.io/name": "juicefs-mount"})
}

func (api *dashboardApi) listCSINodePod() gin.HandlerFunc {
	return api.listPodByLabels(map[string]string{
		"app.kubernetes.io/name": "juicefs-csi-driver",
		"app":                    "juicefs-csi-node",
	})
}

func (api *dashboardApi) listCSIControllerPod() gin.HandlerFunc {
	return api.listPodByLabels(map[string]string{
		"app.kubernetes.io/name": "juicefs-csi-driver",
		"app":                    "juicefs-csi-controller",
	})

}

func (api *dashboardApi) watchAppPod() {
	labelSelector := &v1.LabelSelector{
		MatchExpressions: []v1.LabelSelectorRequirement{{Key: config.UniqueId, Operator: v1.LabelSelectorOpExists}},
	}
	s, err := v1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		log.Fatalf("can't convert label selector %v: %v", labelSelector, err)
	}
	watcher, err := api.k8sClient.CoreV1().Pods("").Watch(context.TODO(), v1.ListOptions{
		LabelSelector: s.String(),
		Watch:         true,
	})
	if err != nil {
		log.Fatalf("can't watch pods by %s: %v", s.String(), err)
	}
	for event := range watcher.ResultChan() {
		api.appPodsLock.Lock()
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			log.Printf("unknown type: %v", event.Object)
			continue
		}
		switch event.Type {
		case watch.Added, watch.Modified, watch.Error:
			api.appPods[string(pod.UID)] = pod
		case watch.Deleted:
			delete(api.appPods, string(pod.UID))
		}
		api.appPodsLock.Unlock()
	}
}

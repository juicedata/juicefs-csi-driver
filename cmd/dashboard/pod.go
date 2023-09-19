package main

import (
	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
)

func (api *dashboardApi) getPodMiddileware() gin.HandlerFunc {
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

func (api *dashboardApi) getPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		pod, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		c.IndentedJSON(200, pod)
	}
}

// func (api *dashboardApi) getPodEvents() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		namespace := c.Param("namespace")
// 		name := c.Param("name")
// 		events, err := api.k8sClient.CoreV1().Events(namespace).List()
// 		if err != nil {
// 			c.String(500, "get pod events: %v", err)
// 			return
// 		}
// 		c.IndentedJSON(200, events)
// 	}
// }

func isPermitted(pod *corev1.Pod) bool {
	_, existUniqueId := pod.Labels[config.UniqueId]
	return existUniqueId ||
		pod.Labels["app.kubernetes.io/name"] == "juicefs-mount" ||
		pod.Labels["app.kubernetes.io/name"] == "juicefs-csi-driver"
}

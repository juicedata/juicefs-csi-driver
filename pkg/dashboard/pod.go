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
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

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

func (api *API) getCSINode(nodeName string) *corev1.Pod {
	api.componentsLock.RLock()
	defer api.componentsLock.RUnlock()
	return api.nodeindex[nodeName]
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

func (api *API) listMountPods() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		pvs := api.listPVsOfPod(c, pod)
		mountPods := make(map[string]*corev1.Pod)
		for pvc, pv := range pvs {
			key := fmt.Sprintf("%s-%s", config.JuiceFSMountPod, pv.Spec.CSI.VolumeHandle)
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

func (api *API) getCSINodeByName() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeName := c.Param("nodeName")
		if nodeName == "" {
			c.String(404, "not found")
			return
		}
		pod := api.getCSINode(nodeName)
		if pod == nil {
			c.String(404, "not found")
			return
		}
		c.IndentedJSON(200, pod)
	}
}

func (api *API) getMountPodOfPV() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pv")
		if !ok {
			c.String(404, "not found")
			return
		}
		pv := obj.(*PVExtended)
		pod := api.getAppPod(pv.Pod)
		if pod == nil {
			c.String(404, "not found")
			return
		}
		key := fmt.Sprintf("%s-%s", config.JuiceFSMountPod, pv.Name)
		mountPodName, ok := pod.Annotations[key]
		if !ok {
			c.String(404, "not found")
			return
		}
		pair := strings.SplitN(mountPodName, string(types.Separator), 2)
		if len(pair) != 2 {
			c.String(500, "invalid mount pod name %s\n", mountPodName)
			return
		}
		api.componentsLock.RLock()
		defer api.componentsLock.RUnlock()
		mountPod, exist := api.mountPods[pair[1]]
		if !exist {
			c.String(404, "not found")
			return
		}
		c.IndentedJSON(200, mountPod)
	}
}

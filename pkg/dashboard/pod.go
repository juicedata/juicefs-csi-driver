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
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func (api *API) listAppPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		descend := c.Query("order") != "ascend"
		nameFilter := c.Query("name")
		namespaceFilter := c.Query("namespace")
		// pvFilter := c.Query("pv")
		// mountpodFilter := c.Query("mountpod")
		// csiNodeFilter := c.Query("csinode")
		api.appPodsLock.RLock()
		pods := make([]*corev1.Pod, 0, api.appIndexes.Len())
		appendPod := func(value any) {
			if pod, ok := api.appPods[value.(types.NamespacedName)]; ok &&
				(nameFilter == "" || strings.Contains(pod.Name, nameFilter)) &&
				(namespaceFilter == "" || strings.Contains(pod.Namespace, namespaceFilter)) {
				pods = append(pods, pod)
			}
		}
		if descend {
			for e := api.appIndexes.Front(); e != nil; e = e.Next() {
				appendPod(e.Value)
			}
		} else {
			for e := api.appIndexes.Back(); e != nil; e = e.Prev() {
				appendPod(e.Value)
			}
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
		events := api.events[types.NamespacedName{
			Namespace: pod.(*corev1.Pod).Namespace,
			Name:      pod.(*corev1.Pod).Name,
		}]
		list := make([]*corev1.Event, 0, len(events))
		for _, e := range events {
			if e.InvolvedObject.UID == pod.(*corev1.Pod).UID {
				list = append(list, e)
			}
		}
		api.eventsLock.RUnlock()
		c.IndentedJSON(200, list)
	}
}

func (api *API) getPVEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		pv, ok := c.Get("pv")
		if !ok {
			c.String(404, "not found")
			return
		}
		api.eventsLock.RLock()
		events := api.events[types.NamespacedName{
			Namespace: pv.(*corev1.PersistentVolume).Namespace,
			Name:      pv.(*corev1.PersistentVolume).Name,
		}]
		list := make([]*corev1.Event, 0, len(events))
		for _, e := range events {
			if e.InvolvedObject.UID == pv.(*corev1.PersistentVolume).UID {
				list = append(list, e)
			}
		}
		api.eventsLock.RUnlock()
		c.IndentedJSON(200, list)
	}
}

func (api *API) getPVCEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		pvc, ok := c.Get("pvc")
		if !ok {
			c.String(404, "not found")
			return
		}
		api.eventsLock.RLock()
		events := api.events[types.NamespacedName{
			Namespace: pvc.(*corev1.PersistentVolumeClaim).Namespace,
			Name:      pvc.(*corev1.PersistentVolumeClaim).Name,
		}]
		list := make([]*corev1.Event, 0, len(events))
		for _, e := range events {
			if e.InvolvedObject.UID == pvc.(*corev1.PersistentVolumeClaim).UID {
				list = append(list, e)
			}
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

func (api *API) listMountPodsOfAppPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		pvs := api.listPVsOfPod(c, pod)
		mountPods := make([]*corev1.Pod, 0)
		for _, pv := range pvs {
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
			mountPods = append(mountPods, mountPod)
		}
		c.IndentedJSON(200, mountPods)
	}
}

func (api *API) listAppPodsOfMountPod() gin.HandlerFunc {
	getUidFunc := func(target string) string {
		pair := strings.Split(target, "volumes/kubernetes.io~csi")
		if len(pair) != 2 {
			return ""
		}

		podDir := strings.TrimSuffix(pair[0], "/")
		index := strings.LastIndex(podDir, "/")
		if index <= 0 {
			return ""
		}
		return podDir[index+1:]
	}

	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		appPods := make([]*corev1.Pod, 0)
		if pod.Annotations != nil {
			api.appPodsLock.Lock()
			allPods := api.appPods
			api.appPodsLock.Unlock()

			podsByUid := make(map[string]*corev1.Pod)
			for _, po := range allPods {
				podsByUid[string(po.UID)] = po
			}
			for _, v := range pod.Annotations {
				uid := getUidFunc(v)
				if uid == "" {
					continue
				}
				if po, ok := podsByUid[uid]; ok {
					appPods = append(appPods, po)
				}
			}
		}
		c.IndentedJSON(200, appPods)
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

func (api *API) getMountPodsOfPV() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pv")
		if !ok {
			c.String(404, "not found")
			return
		}
		pv := obj.(*corev1.PersistentVolume)

		// todo: if unique id is sc name (mount pod shared by sc)
		api.componentsLock.RLock()
		defer api.componentsLock.RUnlock()
		var mountPods = make([]*corev1.Pod, 0)
		for _, pod := range api.mountPods {
			if pod.Labels != nil && pod.Labels[config.PodUniqueIdLabelKey] == pv.Spec.CSI.VolumeHandle {
				mountPods = append(mountPods, pod)
			}
		}
		c.IndentedJSON(200, mountPods)
	}
}

func (api *API) getMountPodsOfPVC() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pvc")
		if !ok {
			c.String(404, "not found")
			return
		}
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvName := api.pairs[types.NamespacedName{
			Namespace: pvc.Namespace,
			Name:      pvc.Name,
		}]
		if pvName == "" {
			c.String(404, "not found")
			return
		}
		pv := api.pvs[pvName]
		if pv == nil {
			c.String(404, "not found")
			return
		}

		// todo: if unique id is sc name (mount pod shared by sc)
		api.componentsLock.RLock()
		defer api.componentsLock.RUnlock()
		var mountPods = make([]*corev1.Pod, 0)
		for _, pod := range api.mountPods {
			if pod.Labels != nil && pod.Labels[config.PodUniqueIdLabelKey] == pv.Spec.CSI.VolumeHandle {
				mountPods = append(mountPods, pod)
			}
		}
		c.IndentedJSON(200, mountPods)
	}
}

func (api *API) getPVOfSC() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("sc")
		if !ok {
			c.String(404, "not found")
			return
		}
		sc := obj.(*storagev1.StorageClass)

		api.pvsLock.RLock()
		defer api.pvsLock.RUnlock()
		var pvs = make([]*corev1.PersistentVolume, 0)
		for _, pv := range api.pvs {
			if pv.Spec.StorageClassName == sc.Name {
				pvs = append(pvs, pv)
			}
		}
		c.IndentedJSON(200, pvs)
	}
}

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
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

type PodExtra struct {
	*corev1.Pod `json:",inline"`
	Pvs         []*corev1.PersistentVolume      `json:"pvs"`
	Pvcs        []*corev1.PersistentVolumeClaim `json:"pvcs"`
	MountPods   []*corev1.Pod                   `json:"mountPods"`
	CsiNode     *corev1.Pod                     `json:"csiNode"`
	Node        *corev1.Node                    `json:"node"`
}

type ListAppPodResult struct {
	Total int         `json:"total"`
	Pods  []*PodExtra `json:"pods"`
}

type ListSysPodResult struct {
	Total int         `json:"total"`
	Pods  []*PodExtra `json:"pods"`
}

func (api *API) listAppPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		pageSize, err := strconv.ParseUint(c.Query("pageSize"), 10, 64)
		if err != nil || pageSize == 0 {
			c.String(400, "invalid page size")
			return
		}
		current, err := strconv.ParseUint(c.Query("current"), 10, 64)
		if err != nil || current == 0 {
			c.String(400, "invalid current page")
			return
		}
		descend := c.Query("order") != "ascend"
		nameFilter := c.Query("name")
		namespaceFilter := c.Query("namespace")
		pvFilter := c.Query("pv")
		mountpodFilter := c.Query("mountpod")
		csiNodeFilter := c.Query("csinode")

		api.appPodsLock.RLock()
		pods := make([]*PodExtra, 0, api.appIndexes.length())
		for name := range api.appIndexes.iterate(c, descend) {
			if pod, ok := api.appPods[name]; ok &&
				(nameFilter == "" || strings.Contains(pod.Name, nameFilter)) &&
				(namespaceFilter == "" || strings.Contains(pod.Namespace, namespaceFilter)) {
				pods = append(pods, &PodExtra{Pod: pod})
			}
		}
		api.appPodsLock.RUnlock()
		if pvFilter != "" || mountpodFilter != "" || csiNodeFilter != "" {
			filterdPods := make([]*PodExtra, 0, len(pods))
			for _, pod := range pods {
				if api.filterPVsOfPod(c, pod, pvFilter) && api.filterMountPodsOfPod(c, pod, mountpodFilter) && api.filterCSINodeOfPod(c, pod, csiNodeFilter) {
					filterdPods = append(filterdPods, pod)
				}
			}
			pods = filterdPods
		}
		result := &ListAppPodResult{len(pods), make([]*PodExtra, 0)}
		startIndex := (current - 1) * pageSize
		if startIndex >= uint64(len(pods)) {
			c.IndentedJSON(200, result)
			return
		}
		endIndex := startIndex + pageSize
		if endIndex > uint64(len(pods)) {
			endIndex = uint64(len(pods))
		}
		result.Pods = pods[startIndex:endIndex]
		for _, pod := range result.Pods {
			if pod.Pvs == nil {
				pod.Pvs = api.listPVsOfPod(c, pod.Pod)
			}
			if pod.MountPods == nil {
				pod.MountPods = api.listMountPodOf(c, pod.Pod)
			}
			if pod.CsiNode == nil {
				pod.CsiNode = api.getCSINode(pod.Spec.NodeName)
			}
			if pod.Spec.NodeName != "" {
				pod.Node = api.getNode(pod.Spec.NodeName)
			}
			pod.Pvcs = api.listPVCsOfPod(c, pod.Pod)
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) filterPVsOfPod(ctx context.Context, pod *PodExtra, filter string) bool {
	if filter == "" {
		return true
	}
	pod.Pvs = api.listPVsOfPod(ctx, pod.Pod)
	for _, pv := range pod.Pvs {
		if strings.Contains(pv.Name, filter) {
			return true
		}
	}
	return false
}

func (api *API) filterMountPodsOfPod(ctx context.Context, pod *PodExtra, filter string) bool {
	if filter == "" {
		return true
	}
	pod.MountPods = api.listMountPodOf(ctx, pod.Pod)
	for _, mountPod := range pod.MountPods {
		if strings.Contains(mountPod.Name, filter) {
			return true
		}
	}
	return false
}

func (api *API) filterCSINodeOfPod(ctx context.Context, pod *PodExtra, filter string) bool {
	if filter == "" {
		return true
	}
	if pod.Spec.NodeName == "" {
		return false
	}
	pod.CsiNode = api.getCSINode(pod.Spec.NodeName)
	if pod.CsiNode == nil {
		return false
	}
	return strings.Contains(pod.CsiNode.Name, filter)
}

func (api *API) listSysPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		pageSize, err := strconv.ParseUint(c.Query("pageSize"), 10, 64)
		if err != nil || pageSize == 0 {
			c.String(400, "invalid page size")
			return
		}
		current, err := strconv.ParseUint(c.Query("current"), 10, 64)
		if err != nil || current == 0 {
			c.String(400, "invalid current page")
			return
		}
		descend := c.Query("order") == "descend"
		nameFilter := c.Query("name")
		namespaceFilter := c.Query("namespace")
		nodeFilter := c.Query("node")
		required := func(pod *corev1.Pod) bool {
			return (nameFilter == "" || strings.Contains(pod.Name, nameFilter)) &&
				(namespaceFilter == "" || strings.Contains(pod.Namespace, namespaceFilter)) &&
				(nodeFilter == "" || strings.Contains(pod.Spec.NodeName, nodeFilter))

		}
		api.componentsLock.RLock()
		pods := make([]*corev1.Pod, 0, api.sysIndexes.length())
		appendPod := func(pod *corev1.Pod) {
			if required(pod) {
				pods = append(pods, pod)
			}
		}
		for name := range api.sysIndexes.iterate(c, descend) {
			if pod, ok := api.mountPods[name]; ok {
				appendPod(pod)
			} else if pod, ok := api.csiNodes[name]; ok {
				appendPod(pod)
			} else if pod, ok := api.controllers[name]; ok {
				appendPod(pod)
			}
		}
		api.componentsLock.RUnlock()
		result := &ListSysPodResult{len(pods), make([]*PodExtra, 0)}
		startIndex := (current - 1) * pageSize
		if startIndex >= uint64(len(pods)) {
			c.IndentedJSON(200, result)
			return
		}
		endIndex := startIndex + pageSize
		if endIndex > uint64(len(pods)) {
			endIndex = uint64(len(pods))
		}
		for i := startIndex; i < endIndex; i++ {
			node := api.getNode(pods[i].Spec.NodeName)
			result.Pods = append(result.Pods, &PodExtra{
				Pod:  pods[i],
				Node: node,
			})
		}
		c.IndentedJSON(200, result)
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

func (api *API) getComponentPod(name types.NamespacedName) (*corev1.Pod, bool) {
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
	return api.csiNodeIndex[nodeName]
}

func (api *API) getNode(nodeName string) *corev1.Node {
	api.nodesLock.RLock()
	defer api.nodesLock.RUnlock()
	return api.nodes[nodeName]
}

func (api *API) getPodMiddileware() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		pod, exist := api.getComponentPod(api.sysNamespaced(name))
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

func (api *API) getPodNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		pod, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		rawPod := pod.(*corev1.Pod)
		nodeName := rawPod.Spec.NodeName
		node, err := api.k8sClient.CoreV1().Nodes().Get(c, nodeName, metav1.GetOptions{})
		if err != nil {
			c.String(500, "Get node %s error %v", nodeName, err)
			return
		}
		c.IndentedJSON(200, node)
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

func (api *API) listMountPodOf(ctx context.Context, pod *corev1.Pod) []*corev1.Pod {
	pvs := api.listPVsOfPod(ctx, pod)
	mountPods := make([]*corev1.Pod, 0)
	for _, pv := range pvs {
		key := fmt.Sprintf("%s-%s", config.JuiceFSMountPod, pv.Spec.CSI.VolumeHandle)
		mountPodName, ok := pod.Annotations[key]
		if !ok {
			klog.V(0).Infof("can't find mount pod name by annotation `%s`\n", key)
			continue
		}
		pair := strings.SplitN(mountPodName, string(types.Separator), 2)
		if len(pair) != 2 {
			klog.V(0).Infof("invalid mount pod name %s\n", mountPodName)
			continue
		}
		api.componentsLock.RLock()
		mountPod, exist := api.mountPods[types.NamespacedName{pair[0], pair[1]}]
		api.componentsLock.RUnlock()
		if !exist {
			klog.V(0).Infof("mount pod %s not found\n", mountPodName)
			continue
		}
		mountPods = append(mountPods, mountPod)
	}
	return mountPods
}

func (api *API) listMountPodsOfAppPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		c.IndentedJSON(200, api.listMountPodOf(c, pod))
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
		pvName, ok := api.pairs[types.NamespacedName{
			Namespace: pvc.Namespace,
			Name:      pvc.Name,
		}]
		if !ok {
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

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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

		pods := make([]*PodExtra, 0, api.appIndexes.length())
		for name := range api.appIndexes.iterate(c, descend) {
			var pod corev1.Pod
			if err := api.cachedReader.Get(c, name, &pod); err == nil &&
				(nameFilter == "" || strings.Contains(pod.Name, nameFilter)) &&
				(namespaceFilter == "" || strings.Contains(pod.Namespace, namespaceFilter)) {
				pods = append(pods, &PodExtra{Pod: &pod})
			}
		}
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
				pod.Pvs, err = api.listPVsOfPod(c, pod.Pod)
				if err != nil {
					klog.Errorf("get pvs of %s error %v", pod.Spec.NodeName, err)
				}
			}
			if pod.MountPods == nil {
				pod.MountPods, err = api.listMountPodOf(c, pod.Pod)
				if err != nil {
					klog.Errorf("get mount pods of %s error %v", pod.Spec.NodeName, err)
				}
			}
			if pod.CsiNode == nil {
				pod.CsiNode, err = api.getCSINode(c, pod.Spec.NodeName)
				if err != nil {
					klog.Errorf("get csi node %s error %v", pod.Spec.NodeName, err)
				}
			}
			if pod.Spec.NodeName != "" {
				var node corev1.Node
				err := api.cachedReader.Get(c, types.NamespacedName{Name: pod.Spec.NodeName}, &node)
				if err != nil {
					klog.Errorf("get node %s error %v", pod.Spec.NodeName, err)
				} else {
					pod.Node = &node
				}
			}
			pod.Pvcs, err = api.listPVCsOfPod(c, pod.Pod)
			if err != nil {
				c.String(500, "list pvcs of pod %s error %v", pod.Name, err)
			}
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) filterPVsOfPod(ctx context.Context, pod *PodExtra, filter string) bool {
	if filter == "" {
		return true
	}
	var err error
	pod.Pvs, err = api.listPVsOfPod(ctx, pod.Pod)
	if err != nil {
		klog.Errorf("get pvs of %s error %v", pod.Spec.NodeName, err)
	}
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
	var err error
	pod.MountPods, err = api.listMountPodOf(ctx, pod.Pod)
	if err != nil {
		klog.Errorf("get mount pods of %s error %v", pod.Spec.NodeName, err)
	}
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
	var err error
	pod.CsiNode, err = api.getCSINode(ctx, pod.Spec.NodeName)
	if err != nil || pod.CsiNode == nil {
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
		pods := make([]*corev1.Pod, 0, api.sysIndexes.length())
		for name := range api.sysIndexes.iterate(c, descend) {
			var pod corev1.Pod
			if err := api.cachedReader.Get(c, name, &pod); err == nil && required(&pod) {
				pods = append(pods, &pod)
			}
		}
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
			var node corev1.Node
			err := api.cachedReader.Get(c, types.NamespacedName{Name: pods[i].Spec.NodeName}, &node)
			if err != nil {
				klog.Errorf("get node %s error %v", pods[i].Spec.NodeName, err)
				continue
			}
			result.Pods = append(result.Pods, &PodExtra{
				Pod:  pods[i],
				Node: &node,
			})
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) listMountPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		var pods corev1.PodList
		s, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "juicefs-mount",
			},
		})
		if err != nil {
			c.String(500, "parse label selector error %v", err)
			return
		}
		err = api.cachedReader.List(c, &pods, &client.ListOptions{
			LabelSelector: s,
		})
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods.Items)
	}
}

func (api *API) listCSINodePod() gin.HandlerFunc {
	return func(c *gin.Context) {
		var pods corev1.PodList
		s, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "juicefs-csi-driver",
				"app":                    "juicefs-csi-node",
			},
		})
		if err != nil {
			c.String(500, "parse label selector error %v", err)
			return
		}
		err = api.cachedReader.List(c, &pods, &client.ListOptions{
			LabelSelector: s,
		})
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods.Items)
	}
}

func (api *API) listCSIControllerPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		var pods corev1.PodList
		s, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "juicefs-csi-driver",
				"app":                    "juicefs-csi-controller",
			},
		})
		if err != nil {
			c.String(500, "parse label selector error %v", err)
			return
		}
		err = api.cachedReader.List(c, &pods, &client.ListOptions{
			LabelSelector: s,
		})
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods.Items)
	}
}

func (api *API) getCSINode(ctx context.Context, nodeName string) (*corev1.Pod, error) {
	api.csiNodeLock.RLock()
	defer api.csiNodeLock.RUnlock()
	name := api.csiNodeIndex[nodeName]
	if name == (types.NamespacedName{}) {
		return nil, nil
	}
	var pod corev1.Pod
	err := api.cachedReader.Get(ctx, name, &pod)
	return &pod, err
}

func (api *API) getPodMiddileware() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		var pod corev1.Pod
		err := api.cachedReader.Get(c, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, &pod)
		if k8serrors.IsNotFound(err) {
			c.AbortWithStatus(404)
			return
		} else if err != nil {
			c.String(500, "get pod error %v", err)
			return
		} else if !isAppPod(&pod) && !isSysPod(&pod) && !api.isAppPodUnready(c, &pod) {
			c.String(404, "not found")
			return
		}
		c.Set("pod", &pod)
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
		p, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := p.(*corev1.Pod)
		list, err := api.client.CoreV1().Events(pod.Namespace).List(c, metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{Kind: "Pod"},
			FieldSelector: fields.SelectorFromSet(fields.Set{
				"involvedObject.uid": string(pod.UID),
			}).String(),
		})
		if err != nil {
			c.String(500, "list events error %v", err)
			return
		}
		c.IndentedJSON(200, list.Items)
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
		var node corev1.Node
		err := api.cachedReader.Get(c, types.NamespacedName{Name: nodeName}, &node)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				c.String(404, "not found")
				return
			}
			c.String(500, "Get node %s error %v", nodeName, err)
			return
		}
		c.IndentedJSON(200, node)
	}
}

func (api *API) getPVEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := c.Get("pv")
		if !ok {
			c.String(404, "not found")
			return
		}
		pv := p.(*corev1.PersistentVolume)
		list, err := api.client.CoreV1().Events("").List(c, metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{Kind: "PersistentVolume"},
			FieldSelector: fields.SelectorFromSet(fields.Set{
				"involvedObject.uid": string(pv.UID),
			}).String(),
		})
		if err != nil {
			c.String(500, "list events error %v", err)
			return
		}
		c.IndentedJSON(200, list.Items)
	}
}

func (api *API) getPVCEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := c.Get("pvc")
		if !ok {
			c.String(404, "not found")
			return
		}
		pvc := p.(*corev1.PersistentVolumeClaim)
		list, err := api.client.CoreV1().Events(pvc.Namespace).List(c, metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{Kind: "PersistentVolumeClaim"},
			FieldSelector: fields.SelectorFromSet(fields.Set{
				"involvedObject.uid": string(pvc.UID),
			}).String(),
		})
		if err != nil {
			c.String(500, "list events error %v", err)
			return
		}
		c.IndentedJSON(200, list.Items)
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

		logs, err := api.client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: container,
		}).DoRaw(c)
		if err != nil {
			c.String(500, "get pod logs of container %s: %v", container, err)
			return
		}
		c.String(200, string(logs))
	}
}

func (api *API) listMountPodOf(ctx context.Context, pod *corev1.Pod) ([]*corev1.Pod, error) {
	pvs, err := api.listPVsOfPod(ctx, pod)
	if err != nil {
		return nil, err
	}
	mountPods := make([]*corev1.Pod, 0)
	for _, pv := range pvs {
		var pods corev1.PodList
		err := api.cachedReader.List(ctx, &pods, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				config.PodUniqueIdLabelKey: pv.Spec.CSI.VolumeHandle,
			}),
		})
		if err != nil {
			continue
		}
		for i := range pods.Items {
			mountPods = append(mountPods, &pods.Items[i])
		}
	}
	return mountPods, nil
}

func (api *API) listMountPodsOfAppPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		pods, err := api.listMountPodOf(c, pod)
		if err != nil {
			c.String(500, "list mount pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods)
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
		var podList corev1.PodList
		err := api.cachedReader.List(c, &podList)
		if err != nil {
			c.String(500, "list pods error %v", err)
		}
		podMap := make(map[string]*corev1.Pod)
		for i := range podList.Items {
			p := &podList.Items[i]
			podMap[string(p.UID)] = p
		}
		appPods := make([]*corev1.Pod, 0)
		if pod.Annotations != nil {
			for _, v := range pod.Annotations {
				uid := getUidFunc(v)
				if uid == "" {
					klog.V(6).Infof("annotation %s skipped", v)
					continue
				}
				if p, ok := podMap[uid]; ok {
					appPods = append(appPods, p)
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
		pod, err := api.getCSINode(c, nodeName)
		if err != nil && !k8serrors.IsNotFound(err) {
			c.String(500, "get csi node %s error %v", nodeName, err)
			return
		}
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
		var pods corev1.PodList
		err := api.cachedReader.List(c, &pods, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				config.PodUniqueIdLabelKey: pv.Spec.CSI.VolumeHandle,
			}),
		})
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods.Items)
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
		var pv corev1.PersistentVolume
		if err := api.cachedReader.Get(c, pvName, &pv); err != nil {
			if k8serrors.IsNotFound(err) {
				c.String(404, "not found")
			} else {
				c.String(500, "get pv %s error %v", pvName, err)
			}
			return
		}

		// todo: if unique id is sc name (mount pod shared by sc)
		var pods corev1.PodList
		err := api.cachedReader.List(c, &pods, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				config.PodUniqueIdLabelKey: pv.Spec.CSI.VolumeHandle,
			}),
		})
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods.Items)
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
		var pvList corev1.PersistentVolumeList
		err := api.cachedReader.List(c, &pvList)
		if err != nil {
			c.String(500, "list pvs error %v", err)
			return
		}
		var pvs []*corev1.PersistentVolume
		for i := range pvList.Items {
			pv := &pvList.Items[i]
			if pv.Spec.StorageClassName == sc.Name {
				pvs = append(pvs, pv)
			}
		}
		c.IndentedJSON(200, pvs)
	}
}

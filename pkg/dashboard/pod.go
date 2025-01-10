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

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var podLog = klog.NewKlogr().WithName("pod")

func (api *API) listAppPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := api.podSvc.ListAppPods(c)
		if err != nil {
			return
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) listSysPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := api.podSvc.ListSysPods(c)
		if err != nil {
			return
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) listCSINodePod() gin.HandlerFunc {
	return func(c *gin.Context) {
		var targetPod *corev1.Pod
		if v, ok := c.Get("pod"); ok {
			targetPod = v.(*corev1.Pod)
		}
		if targetPod.Labels["app.kubernetes.io/name"] == "juicefs-csi-driver" {
			return
		}
		pods, err := api.podSvc.ListCSINodePod(c, targetPod.Spec.NodeName)
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods)
	}
}

func (api *API) getCSINode(ctx context.Context, nodeName string) (*corev1.Pod, error) {
	pods, err := api.podSvc.ListCSINodePod(ctx, nodeName)
	if err != nil {
		return nil, err
	}
	return &pods[0], err
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
		}

		if utils.IsAppPod(&pod) || utils.IsSysPod(&pod) {
			c.Set("pod", &pod)
			return
		}

		c.String(404, "not found")
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

func (api *API) getPodLatestImage() gin.HandlerFunc {
	return func(c *gin.Context) {
		po, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		rawPod := po.(*corev1.Pod)
		if rawPod.Labels[common.PodTypeKey] != common.PodTypeValue {
			c.String(400, "pod %s is not a mount pod", rawPod.Name)
			return
		}
		if err := config.LoadFromConfigMap(c, api.client); err != nil {
			c.String(500, "Load config from configmap error: %v", err)
			return
		}
		setting, err := config.GenSettingAttrWithMountPod(c, api.client, rawPod)
		if err != nil {
			c.String(500, "generate pod attribute error: %v", err)
			return
		}
		c.IndentedJSON(200, setting.Attr.Image)
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
		result, err := api.eventSvc.ListEvents(c, pod.Namespace, "Pod", string(pod.UID))
		if err != nil {
			c.String(500, "list events error %v", err)
			return
		}
		c.IndentedJSON(200, result)
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
		download := c.Query("download")
		if download == "true" {
			c.Header("Content-Disposition", "attachment; filename="+pod.Name+"_"+container+".log")
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

func (api *API) listMountPodsOfAppPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		if utils.IsCsiNode(pod) {
			mountPods, err := api.podSvc.ListNodeMountPods(c, pod.Spec.NodeName)
			if err != nil {
				c.String(500, "list mount pods error %v", err)
				return
			}
			c.IndentedJSON(200, mountPods)
			return
		}
		pods, err := api.podSvc.ListAppPodMountPods(c, pod)
		if err != nil {
			c.String(500, "list mount pods error %v", err)
			return
		}
		c.IndentedJSON(200, pods)
	}
}

func (api *API) listAppPodsOfMountPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		obj, ok := c.Get("pod")
		if !ok {
			c.String(404, "not found")
			return
		}
		pod := obj.(*corev1.Pod)
		result, err := api.podSvc.ListMountPodAppPods(c, pod)
		if err != nil {
			c.String(500, "list app pods error %v", err)
			return
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) watchPodLogs() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		container := c.Param("container")
		if err := api.podSvc.WatchPodLogs(c, namespace, name, container); err != nil {
			podLog.Error(err, "Failed to watch pod logs")
			return
		}
	}
}

func (api *API) execPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		container := c.Param("container")
		api.podSvc.ExecPod(c, namespace, name, container)
	}
}

func (api *API) watchMountPodAccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		container := c.Param("container")
		api.podSvc.WatchMountPodAccessLog(c, namespace, name, container)
	}
}

func (api *API) debugPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		container := c.Param("container")
		api.podSvc.DebugPod(c, namespace, name, container)
	}
}

func (api *API) warmupPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		container := c.Param("container")
		api.podSvc.WarmupPod(c, namespace, name, container)
	}
}

func (api *API) downloadDebugFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		container := common.MountContainerName
		c.Header("Content-Disposition", "attachment; filename="+namespace+"_"+name+"_"+"debug.zip")
		if err := api.podSvc.DownloadDebugFile(c, namespace, name, container); err != nil {
			podLog.Error(err, "Failed to download debug file")
			return
		}
	}
}

func (api *API) smoothUpgrade() gin.HandlerFunc {
	return func(c *gin.Context) {
		websocket.Handler(func(ws *websocket.Conn) {
			defer ws.Close()
			ctx, cancel := context.WithCancel(c.Request.Context())
			defer cancel()
			terminal := resource.NewTerminalSession(ctx, ws, resource.EndOfText)

			namespace := c.Param("namespace")
			name := c.Param("name")

			mountpod, err := api.client.CoreV1().Pods(namespace).Get(c, name, metav1.GetOptions{})
			if err != nil {
				klog.Error("Failed to get mount pod: ", err)
				_, _ = ws.Write([]byte("Failed to get mount pod: " + err.Error()))
				return
			}
			recreate := c.Query("recreate")
			podLog.Info("upgrade juicefs-csi-driver", "pod", mountpod.Name, "recreate", recreate)

			csiNode, err := api.getCSINode(c, mountpod.Spec.NodeName)
			if err != nil {
				podLog.Error(err, "get csi node error", "node", mountpod.Spec.NodeName)
				_, _ = ws.Write([]byte("get csi node error: " + err.Error()))
				c.String(500, "get csi node error %v", err)
				return
			}
			if csiNode == nil {
				_, _ = ws.Write([]byte("csi node not found"))
				c.String(404, "csi node not found")
				return
			}

			podLog.Info("Start to upgrade juicefs-csi-driver", "pod", mountpod.Name, "recreate", recreate)
			cmds := []string{"juicefs-csi-driver", "upgrade", mountpod.Name}
			if recreate == "true" {
				cmds = append(cmds, "--recreate")
			}
			podLog.Info("cmds", "cmds", cmds)

			if err := resource.ExecInPod(
				ctx,
				api.client, api.kubeconfig, terminal, csiNode.Namespace, csiNode.Name, "juicefs-plugin", cmds); err != nil {
				podLog.Error(err, "Failed to start process")
				return
			}
		}).ServeHTTP(c.Writer, c.Request)
	}
}

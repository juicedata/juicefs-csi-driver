/*
 Copyright 2024 Juicedata Inc

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
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

func (api *API) getCSIConfig() gin.HandlerFunc {
	cmName := os.Getenv("JUICEFS_CONFIG_NAME")
	if cmName == "" {
		cmName = "juicefs-csi-driver-config"
	}
	return func(c *gin.Context) {
		cm, err := api.client.CoreV1().ConfigMaps(api.sysNamespace).Get(c, cmName, metav1.GetOptions{})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, cm)
	}
}

func (api *API) putCSIConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var cm corev1.ConfigMap
		if err := c.ShouldBindJSON(&cm); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		cmName := os.Getenv("JUICEFS_CONFIG_NAME")
		if cmName == "" {
			cmName = "juicefs-csi-driver-config"
		}
		if cm.Name != cmName {
			c.JSON(400, gin.H{"error": "invalid config map name"})
			return
		}
		_, err := api.client.CoreV1().ConfigMaps(api.sysNamespace).Update(c, &cm, metav1.UpdateOptions{})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
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
		csiNodeList, err := api.client.CoreV1().Pods(api.sysNamespace).List(c, metav1.ListOptions{LabelSelector: s.String()})
		if err != nil {
			c.String(500, "list csi node error %v", err)
			return
		}
		for _, pod := range csiNodeList.Items {
			pod.Annotations["juicefs/update-time"] = metav1.Now().Format("2006-01-02T15:04:05Z")
			_, err = api.client.CoreV1().Pods(api.sysNamespace).Update(c, &pod, metav1.UpdateOptions{})
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
		}
		c.JSON(200, cm)
	}
}

func (api *API) getCSIConfigDiff() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeName := c.Query("nodeName")
		uniqueIdsStr := c.Query("uniqueIds")
		uniqueIds := strings.FieldsFunc(uniqueIdsStr, func(r rune) bool {
			return r == '/'
		})
		// gen k8s client
		k8sClient, err := k8sclient.NewClientWithConfig(api.kubeconfig)
		if err != nil {
			c.String(500, "Could not create k8s client: %v", err)
			return
		}
		// get all mount pods
		var pods corev1.PodList
		ls := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "juicefs-mount",
			},
		}
		if len(uniqueIds) != 0 {
			ls.MatchExpressions = []metav1.LabelSelectorRequirement{{
				Key:      common.PodUniqueIdLabelKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   uniqueIds,
			}}
		}
		s, _ := metav1.LabelSelectorAsSelector(ls)
		listOptions := client.ListOptions{
			LabelSelector: s,
		}
		if nodeName != "" {
			fieldSelector := fields.Set{"spec.nodeName": nodeName}.AsSelector()
			listOptions.FieldSelector = fieldSelector
		}
		err = api.cachedReader.List(c, &pods, &listOptions)
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}

		// load config
		if err := config.LoadFromConfigMap(c, k8sClient); err != nil {
			c.String(500, "Load config from configmap error: %v", err)
			return
		}

		var needUpdatePods []corev1.Pod
		for _, pod := range pods.Items {
			po := pod
			diff, err := DiffConfig(c, k8sClient, &po)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			if diff {
				needUpdatePods = append(needUpdatePods, po)
			}
		}

		c.JSON(200, needUpdatePods)
	}
}

func DiffConfig(ctx context.Context, client *k8sclient.K8sClient, pod *corev1.Pod) (bool, error) {
	setting, err := config.GenSettingAttrWithMountPod(ctx, client, pod)
	if err != nil {
		return false, err
	}
	return setting.HashVal != pod.Labels[common.PodJuiceHashLabelKey], nil
}

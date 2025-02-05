/*
 Copyright 2025 Juicedata Inc

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

package pods

import (
	"context"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type PodExtra struct {
	*corev1.Pod `json:",inline"`
	Pvs         []corev1.PersistentVolume      `json:"pvs,omitempty"`
	Pvcs        []corev1.PersistentVolumeClaim `json:"pvcs,omitempty"`
	MountPods   []corev1.Pod                   `json:"mountPods,omitempty"`
	CsiNode     *corev1.Pod                    `json:"csiNode,omitempty"`
	Node        *corev1.Node                   `json:"node,omitempty"`
}

type ListAppPodResult struct {
	Total    int        `json:"total,omitempty"`
	Continue string     `json:"continue,omitempty"`
	Pods     []PodExtra `json:"pods"`
}

type ListSysPodResult struct {
	Total    int        `json:"total"`
	Continue string     `json:"continue"`
	Pods     []PodExtra `json:"pods"`
}

type PodDiff struct {
	Pod        corev1.Pod           `json:"pod"`
	OldConfig  config.MountPodPatch `json:"oldConfig"`
	OldSetting *config.JfsSetting   `json:"oldSetting,omitempty"`
	NewConfig  config.MountPodPatch `json:"newConfig"`
	NewSetting *config.JfsSetting   `json:"newSetting,omitempty"`
}

type PodService interface {
	ListAppPods(ctx *gin.Context) (*ListAppPodResult, error)
	ListSysPods(ctx *gin.Context) (*ListSysPodResult, error)
	ListCSINodePod(ctx context.Context, nodeName string) ([]corev1.Pod, error)
	ListPodPVs(ctx context.Context, pod *corev1.Pod) ([]corev1.PersistentVolume, error)
	ListPodPVCs(ctx context.Context, pod *corev1.Pod) ([]corev1.PersistentVolumeClaim, error)
	ListAppPodMountPods(ctx context.Context, pod *corev1.Pod) ([]corev1.Pod, error)
	ListNodeMountPods(ctx context.Context, nodeName string) ([]corev1.Pod, error)
	ListMountPodAppPods(ctx context.Context, mountPod *corev1.Pod) ([]corev1.Pod, error)
	ListBatchPods(c *gin.Context, conf *config.BatchConfig) ([]corev1.Pod, error)
	ListUpgradePods(c *gin.Context, uniqueId string, nodeName string, recreate bool) ([]corev1.Pod, error)

	ExecPod(c *gin.Context, namespace, name, ontainer string)
	WatchPodLogs(c *gin.Context, namespace, name, container string) error
	WatchMountPodAccessLog(c *gin.Context, namespace, name, container string)
	DebugPod(c *gin.Context, namespace, name, container string)
	WarmupPod(c *gin.Context, namespace, name, container string)
	DownloadDebugFile(c *gin.Context, namespace, name, container string) error
}

func NewPodService(client client.Client, k8sClient *k8sclient.K8sClient, kubeconfig *rest.Config, enableManager bool) PodService {
	svc := &podService{
		k8sClient:    k8sClient,
		client:       client,
		kubeconfig:   kubeconfig,
		sysNamespace: config.Namespace,
	}
	if enableManager {
		return &CachePodService{
			podService:   svc,
			csiNodeIndex: make(map[string]types.NamespacedName),
			sysIndexes:   utils.NewTimeIndexes[corev1.Pod](),
			appIndexes:   utils.NewTimeIndexes[corev1.Pod](),
		}
	}

	csiNodes, err := svc.ListCSINodePod(context.TODO(), "")
	if err != nil || len(csiNodes) == 0 {
		podLog.Error(err, "can't list csi node pods")
		return svc
	}
	// get csi pod spec
	config.CSIPod = csiNodes[0]
	return svc
}

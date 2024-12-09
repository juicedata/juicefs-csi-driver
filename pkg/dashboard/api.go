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
	"sync"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type API struct {
	sysNamespace string
	// for cached resources
	cachedReader client.Reader
	// for logs and events
	client     kubernetes.Interface
	k8sclient  *k8sclient.K8sClient
	kubeconfig *rest.Config

	csiNodeLock  sync.RWMutex
	csiNodeIndex map[string]types.NamespacedName
	sysIndexes   *timeOrderedIndexes[corev1.Pod]
	appIndexes   *timeOrderedIndexes[corev1.Pod]
	pvIndexes    *timeOrderedIndexes[corev1.PersistentVolume]
	pvcIndexes   *timeOrderedIndexes[corev1.PersistentVolumeClaim]
	pairLock     sync.RWMutex
	pairs        map[types.NamespacedName]types.NamespacedName
}

func NewAPI(ctx context.Context, sysNamespace string, cachedReader client.Reader, config *rest.Config) *API {
	// gen k8s client
	k8sClient, err := k8sclient.NewClientWithConfig(*config)
	if err != nil {
		return nil
	}
	api := &API{
		sysNamespace: sysNamespace,
		cachedReader: cachedReader,
		client:       kubernetes.NewForConfigOrDie(config),
		k8sclient:    k8sClient,
		csiNodeIndex: make(map[string]types.NamespacedName),
		sysIndexes:   newTimeIndexes[corev1.Pod](),
		appIndexes:   newTimeIndexes[corev1.Pod](),
		pvIndexes:    newTimeIndexes[corev1.PersistentVolume](),
		pvcIndexes:   newTimeIndexes[corev1.PersistentVolumeClaim](),
		pairs:        make(map[types.NamespacedName]types.NamespacedName),
		kubeconfig:   config,
	}
	return api
}

func (api *API) Handle(group *gin.RouterGroup) {
	group.GET("/debug/status", api.debugAPIStatus())
	group.GET("/pods", api.listAppPod())
	group.GET("/syspods", api.listSysPod())
	group.GET("/mountpods", api.listMountPod())
	group.GET("/csi-nodes", api.listCSINodePod())
	group.GET("/controllers", api.listCSIControllerPod())
	group.GET("/pvs", api.listPVsHandler())
	group.GET("/pvcs", api.listPVCsHandler())
	group.GET("/storageclasses", api.listSCsHandler())
	group.GET("/csi-node/:nodeName", api.getCSINodeByName())
	group.GET("/config", api.getCSIConfig())
	group.PUT("/config", api.putCSIConfig())
	group.GET("/config/diff", api.getCSIConfigDiff())
	group.GET("/nodes", api.getNodes())
	podGroup := group.Group("/pod/:namespace/:name", api.getPodMiddileware())
	podGroup.GET("/", api.getPodHandler())
	podGroup.GET("/latestimage", api.getPodLatestImage())
	podGroup.GET("/events", api.getPodEvents())
	podGroup.GET("/logs/:container", api.getPodLogs())
	podGroup.GET("/pvs", api.listPodPVsHandler())
	podGroup.GET("/pvcs", api.listPodPVCsHandler())
	podGroup.GET("/mountpods", api.listMountPodsOfAppPod())
	podGroup.GET("/apppods", api.listAppPodsOfMountPod())
	podGroup.GET("/csi-nodes", api.listCSINodePod())
	podGroup.GET("/node", api.getPodNode())
	podGroup.GET("/downloadDebugFile", api.downloadDebugFile())
	pvGroup := group.Group("/pv/:name", api.getPVMiddileware())
	pvGroup.GET("/", api.getPVHandler())
	pvGroup.GET("/mountpods", api.getMountPodsOfPV())
	pvGroup.GET("/events", api.getPVEvents())
	pvcGroup := group.Group("/pvc/:namespace/:name", api.getPVCMiddileware())
	pvcGroup.GET("/", api.getPVCHandler())
	pvcGroup.GET("/uniqueid", api.getPVCWithPVHandler())
	pvcGroup.GET("/mountpods", api.getMountPodsOfPVC())
	pvcGroup.GET("/events", api.getPVCEvents())
	scGroup := group.Group("/storageclass/:name", api.getSCMiddileware())
	scGroup.GET("/", api.getSCHandler())
	scGroup.GET("/pvs", api.getPVOfSC())
	batchGroup := group.Group("/batch")
	batchGroup.GET("/job", api.getUpgradeStatus())
	batchGroup.GET("/plan", api.getBatchPlan())
	batchGroup.DELETE("/job", api.clearUpgradeStatus())
	batchGroup.POST("/upgrade", api.upgradePods())
	batchGroup.GET("/job/logs", api.getUpgradeJobLog())

	websocketAPI := group.Group("/ws")
	websocketAPI.GET("/batch/upgrade/logs", api.watchUpgradeJobLog())
	websocketAPI.GET("/pod/:namespace/:name/:container/logs", api.watchPodLogs())
	// only for mountpod
	websocketAPI.GET("/pod/:namespace/:name/:container/accesslog", api.watchMountPodAccessLog())
	websocketAPI.GET("/pod/:namespace/:name/:container/debug", api.debugPod())
	websocketAPI.GET("/pod/:namespace/:name/upgrade", api.smoothUpgrade())
	websocketAPI.GET("/pod/:namespace/:name/:container/warmup", api.warmupPod())
	websocketAPI.GET("/pod/:namespace/:name/:container/exec", api.execPod())
}

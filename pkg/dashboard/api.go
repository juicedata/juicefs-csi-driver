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
	batchv1 "k8s.io/api/batch/v1"
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
	mgrClient    client.Client
	// for logs and events
	client     kubernetes.Interface
	k8sclient  *k8sclient.K8sClient
	kubeconfig *rest.Config

	csiNodeLock   sync.RWMutex
	csiNodeIndex  map[string]types.NamespacedName
	sysIndexes    *timeOrderedIndexes[corev1.Pod]
	appIndexes    *timeOrderedIndexes[corev1.Pod]
	pvIndexes     *timeOrderedIndexes[corev1.PersistentVolume]
	pvcIndexes    *timeOrderedIndexes[corev1.PersistentVolumeClaim]
	jobsIndexes   *timeOrderedIndexes[batchv1.Job]
	secretIndexes *timeOrderedIndexes[corev1.Secret]
	pairLock      sync.RWMutex
	pairs         map[types.NamespacedName]types.NamespacedName
}

func NewAPI(ctx context.Context, sysNamespace string, client client.Client, config *rest.Config) *API {
	// gen k8s client
	k8sClient, err := k8sclient.NewClientWithConfig(*config)
	if err != nil {
		return nil
	}
	api := &API{
		sysNamespace:  sysNamespace,
		cachedReader:  client,
		mgrClient:     client,
		client:        kubernetes.NewForConfigOrDie(config),
		k8sclient:     k8sClient,
		csiNodeIndex:  make(map[string]types.NamespacedName),
		sysIndexes:    newTimeIndexes[corev1.Pod](),
		appIndexes:    newTimeIndexes[corev1.Pod](),
		pvIndexes:     newTimeIndexes[corev1.PersistentVolume](),
		pvcIndexes:    newTimeIndexes[corev1.PersistentVolumeClaim](),
		jobsIndexes:   newTimeIndexes[batchv1.Job](),
		secretIndexes: newTimeIndexes[corev1.Secret](),
		pairs:         make(map[types.NamespacedName]types.NamespacedName),
		kubeconfig:    config,
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
	group.GET("/pvcs/uniqueids/:uniqueid", api.getPVCByUniqueId())
	group.GET("/storageclasses", api.listSCsHandler())
	group.GET("/cachegroups", api.listCacheGroups())
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
	batchGroup := group.Group("/batch/upgrade")
	batchGroup.GET("/plan", api.getBatchPlan())
	batchGroup.GET("/jobs", api.listUpgradeJobs())
	batchGroup.POST("/jobs", api.createUpgradeJob())
	batchGroup.GET("/jobs/:jobName", api.getUpgradeJob())
	batchGroup.DELETE("/jobs/:jobName", api.deleteUpgradeJob())
	batchGroup.PUT("/jobs/:jobName", api.updateUpgradeJob())
	batchGroup.GET("/jobs/:jobName/logs", api.getUpgradeJobLog())
	cgGroup := group.Group("/cachegroup/:namespace/:name")
	cgGroup.GET("/", api.getCacheGroup())
	cgGroup.POST("/", api.createCacheGroup())
	cgGroup.PUT("/", api.updateCacheGroup())
	cgGroup.DELETE("/", api.deleteCacheGroup())
	cgWorkersGroup := group.Group("/cachegroup/:namespace/:name/workers")
	cgWorkersGroup.GET("/", api.listCacheGroupWorkers())
	cgWorkersGroup.POST("/", api.addWorker())
	cgWorkersGroup.DELETE("/", api.removeWorker())
	cgWorkersGroup.GET("/:workerName/cacheBytes", api.getCacheWorkerBytes())

	websocketAPI := group.Group("/ws")
	websocketAPI.GET("/batch/upgrade/jobs/:jobName/logs", api.watchUpgradeJobLog())
	websocketAPI.GET("/pod/:namespace/:name/:container/logs", api.watchPodLogs())
	// only for mountpod
	websocketAPI.GET("/pod/:namespace/:name/:container/accesslog", api.watchMountPodAccessLog())
	websocketAPI.GET("/pod/:namespace/:name/:container/debug", api.debugPod())
	websocketAPI.GET("/pod/:namespace/:name/upgrade", api.smoothUpgrade())
	websocketAPI.GET("/pod/:namespace/:name/:container/warmup", api.warmupPod())
	websocketAPI.GET("/pod/:namespace/:name/:container/exec", api.execPod())
}

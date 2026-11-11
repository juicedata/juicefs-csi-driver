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
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/services/events"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/services/jobs"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/services/pods"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/services/pvcs"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/services/pvs"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/services/secrets"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type API struct {
	sysNamespace            string
	enableManager           bool
	secretManagementEnabled bool
	// for cached resources
	cachedReader client.Reader
	mgrClient    client.Client
	client       *k8sclient.K8sClient
	kubeconfig   *rest.Config

	podSvc    pods.PodService
	pvSvc     pvs.PVService
	pvcSvc    pvcs.PVCService
	secretSvc secrets.SecretService
	jobSvc    jobs.JobService
	eventSvc  events.EventService
}

func NewAPI(ctx context.Context, sysNamespace string, client client.Client, config *rest.Config, enableManager bool) *API {
	// gen k8s client
	k8sClient, err := k8sclient.NewClientWithConfig(*config)
	if err != nil {
		return nil
	}
	client = &readTimeoutClient{Client: client}
	api := &API{
		sysNamespace:            sysNamespace,
		enableManager:           enableManager,
		secretManagementEnabled: !strings.EqualFold(strings.TrimSpace(os.Getenv("ENABLE_SECRET_MANAGEMENT")), "false"),
		cachedReader:            client,
		mgrClient:               client,
		client:                  k8sClient,
		kubeconfig:              config,
		pvSvc:                   pvs.NewPVService(client, enableManager),
		podSvc:                  pods.NewPodService(client, k8sClient, config, enableManager),
		pvcSvc:                  pvcs.NewPVCService(client, enableManager),
		eventSvc:                events.NewEventService(k8sClient),
		secretSvc:               secrets.NewSecretService(client, enableManager),
		jobSvc:                  jobs.NewJobService(client, enableManager),
	}
	return api
}

func (api *API) Handle(group *gin.RouterGroup) {
	group.GET("/pods", api.listAppPod())
	group.GET("/syspods", api.listSysPod())
	group.GET("/pvs", api.listPVsHandler())
	group.POST("/pvs", api.createStorageResource("pv"))
	group.GET("/pvcs", api.listPVCsHandler())
	group.POST("/pvcs", api.createStorageResource("pvc"))
	group.GET("/pvcs/basic", api.listPVCsBasicHandler())
	group.GET("/storageclasses", api.listSCsHandler())
	group.GET("/storageclasses/basic", api.listSCsBasic())
	group.POST("/storageclasses", api.createStorageResource("storageclass"))
	group.GET("/storage-driver", api.getStorageDriver())
	secretGroup := group.Group("", func(c *gin.Context) {
		if !api.secretManagementEnabled {
			c.AbortWithStatus(403)
			return
		}
		c.Next()
	})
	secretGroup.GET("/secrets", api.listJuiceSecrets())
	secretGroup.GET("/secrets/basic", api.listSecretNames())
	secretGroup.POST("/secrets", api.createStorageResource("secret"))
	secretGroup.GET("/secret/:namespace/:name", api.getJuiceSecret())
	secretGroup.PUT("/secret/:namespace/:name", api.updateStorageResource("secret"))
	secretGroup.DELETE("/secret/:namespace/:name", api.deleteStorageResource("secret"))
	group.GET("/cachegroups", api.listCacheGroups())
	group.GET("/warmups", api.listWarmUps())
	group.POST("/warmups", api.createWarmUp())
	group.GET("/warmups/:namespace/:name", api.getWarmUp())
	group.DELETE("/warmups/:namespace/:name", api.deleteWarmUp())
	group.GET("/warmups/:namespace/:name/pods", api.listWarmUpPods())
	group.GET("/syncs", api.listSyncs())
	group.POST("/syncs", api.createSync())
	group.GET("/syncs/:namespace/:name", api.getSync())
	group.DELETE("/syncs/:namespace/:name", api.deleteSync())
	group.GET("/syncs/:namespace/:name/pods", api.listSyncPods())
	group.GET("/config", api.getCSIConfig())
	group.PUT("/config", api.putCSIConfig())
	group.GET("/nodes", api.getNodes())
	group.GET("/version", api.getVersion())

	group.GET("/pvcs/uniqueids/:uniqueid", api.getPVCByUniqueId())
	group.GET("/config/pvcs", api.listPVCWithSelectorHandler())
	group.POST("/config/pvcs/selector", api.listPVCWithSelectorHandler())
	group.GET("/config/diff", api.getCSIConfigDiff())

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
	podGroup.GET("/downloadAllDebugInfo", api.downloadDebugInfo())

	pvGroup := group.Group("/pv/:name", api.getPVMiddileware())
	pvGroup.GET("/", api.getPVHandler())
	pvGroup.PUT("/", api.updateStorageResource("pv"))
	pvGroup.DELETE("/", api.deleteStorageResource("pv"))
	pvGroup.GET("/mountpods", api.getMountPodsOfPV())
	pvGroup.GET("/events", api.getPVEvents())

	pvcGroup := group.Group("/pvc/:namespace/:name", api.getPVCMiddileware())
	pvcGroup.GET("/", api.getPVCHandler())
	pvcGroup.PUT("/", api.updateStorageResource("pvc"))
	pvcGroup.DELETE("/", api.deleteStorageResource("pvc"))
	pvcGroup.GET("/uniqueid", api.getPVCWithPVHandler())
	pvcGroup.GET("/mountpods", api.getMountPodsOfPVC())
	pvcGroup.GET("/events", api.getPVCEvents())

	scGroup := group.Group("/storageclass/:name", api.getSCMiddileware())
	scGroup.GET("/", api.getSCHandler())
	scGroup.PUT("/", api.updateStorageResource("storageclass"))
	scGroup.DELETE("/", api.deleteStorageResource("storageclass"))
	scGroup.GET("/pvs", api.getPVOfSC())

	batchGroup := group.Group("/batch/upgrade")
	batchGroup.GET("/jobs", api.listUpgradeJobs())
	batchGroup.GET("/sidecar-targets", api.listSidecarUpgradeTargets())
	batchGroup.POST("/jobs", api.createUpgradeJob())
	batchGroup.GET("/jobs/:jobName", api.getUpgradeJob())
	batchGroup.GET("/jobs/:jobName/diff", api.getUpgradeJobDiff())
	batchGroup.DELETE("/jobs/:jobName", api.deleteUpgradeJob())
	batchGroup.PUT("/jobs/:jobName", api.updateUpgradeJob())
	batchGroup.GET("/jobs/:jobName/logs", api.getUpgradeJobLog())

	cgGroup := group.Group("/cachegroup/:namespace/:name")
	cgGroup.GET("/", api.getCacheGroup())
	cgGroup.GET("/events", api.getCacheGroupEvents())
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
	websocketAPI.GET("/pod/:namespace/:name/:container/stats", api.statsPod())
	websocketAPI.GET("/pod/:namespace/:name/:container/exec", api.execPod())
}

// getVersion handles GET /api/v1/version requests
func (api *API) getVersion() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.IndentedJSON(200, struct {
			driver.VersionInfo
			SecretManagementEnabled bool `json:"secretManagementEnabled"`
		}{driver.GetVersion(), api.secretManagementEnabled})
	}
}

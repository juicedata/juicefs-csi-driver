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
	"context"
	"sync"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type API struct {
	sysNamespace string
	k8sClient    *k8sclient.K8sClient

	componentsLock sync.RWMutex
	mountPods      map[string]*corev1.Pod
	csiNodes       map[string]*corev1.Pod
	nodeindex      map[string]*corev1.Pod
	controllers    map[string]*corev1.Pod

	appPodsLock sync.RWMutex
	appPods     map[types.NamespacedName]*corev1.Pod

	eventsLock sync.RWMutex
	events     map[string]map[string]*corev1.Event

	pvsLock sync.RWMutex
	pvs     map[string]*corev1.PersistentVolume
	pvcs    map[types.NamespacedName]*corev1.PersistentVolumeClaim
	pairs   map[types.NamespacedName]string
}

func NewAPI(ctx context.Context, sysNamespace string, k8sClient *k8sclient.K8sClient) *API {
	api := &API{
		sysNamespace: sysNamespace,
		k8sClient:    k8sClient,
		mountPods:    make(map[string]*corev1.Pod),
		csiNodes:     make(map[string]*corev1.Pod),
		nodeindex:    make(map[string]*corev1.Pod),
		controllers:  make(map[string]*corev1.Pod),
		appPods:      make(map[types.NamespacedName]*corev1.Pod),
		events:       make(map[string]map[string]*corev1.Event),
		pvs:          make(map[string]*corev1.PersistentVolume),
		pvcs:         make(map[types.NamespacedName]*corev1.PersistentVolumeClaim),
		pairs:        make(map[types.NamespacedName]string),
	}
	go api.watchComponents(ctx)
	go api.watchAppPod(ctx)
	go api.watchRelatedPV(ctx)
	go api.watchRelatedPVC(ctx)
	go api.watchPodEvents(ctx)
	go api.cleanupPodEvents(ctx)
	return api
}

func (api *API) Handle(group *gin.RouterGroup) {
	group.GET("/debug/status", api.debugAPIStatus())
	group.GET("/pods", api.listAppPod())
	group.GET("/mountpods", api.listMountPod())
	group.GET("/csi-nodes", api.listCSINodePod())
	group.GET("/controllers", api.listCSIControllerPod())
	group.GET("/pvs", api.listPVsHandler())
	group.GET("/pvcs", api.listPVCsHandler())
	group.GET("/storageclasses", api.listSCsHandler())
	group.GET("/csi-node/:nodeName", api.getCSINodeByName())
	podGroup := group.Group("/pod/:namespace/:name", api.getPodMiddileware())
	podGroup.GET("/", api.getPodHandler())
	podGroup.GET("/events", api.getPodEvents())
	podGroup.GET("/logs/:container", api.getPodLogs())
	podGroup.GET("/pvs", api.listPodPVsHandler())
	podGroup.GET("/pvcs", api.listPodPVCsHandler())
	podGroup.GET("/mountpods", api.listMountPods())
	pvGroup := group.Group("/pv/:name", api.getPVMiddileware())
	pvGroup.GET("/", api.getPVHandler())
	pvGroup.GET("/mountpods", api.getMountPodsOfPV())
	pvcGroup := group.Group("/pvc/:namespace/:name", api.getPVCMiddileware())
	pvcGroup.GET("/", api.getPVCHandler())
	pvcGroup.GET("/mountpods", api.getMountPodsOfPVC())
	scGroup := group.Group("/storageclass/:name", api.getSCMiddileware())
	scGroup.GET("/", api.getSCHandler())
}

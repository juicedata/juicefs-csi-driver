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
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/types"
)

type APIStatus struct {
	SysNamespace string
	MountPods    []string
	CsiNodes     []string
	Controllers  []string
	SysIndexes   []types.NamespacedName
	AppPods      []types.NamespacedName
	AppIndexes   []types.NamespacedName
	Nodeindex    map[string]types.NamespacedName
	Events       map[string]int
	Pvs          map[string]types.NamespacedName
	PvIndexes    []types.NamespacedName
	PvcIndexes   []types.NamespacedName
}

func (api *API) debugAPIStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		status := &APIStatus{
			SysNamespace: api.sysNamespace,
			Nodeindex:    make(map[string]types.NamespacedName),
			Events:       make(map[string]int),
			Pvs:          make(map[string]types.NamespacedName),
		}
		api.componentsLock.RLock()
		for k := range api.mountPods {
			status.MountPods = append(status.MountPods, k.String())
		}
		for k := range api.csiNodes {
			status.CsiNodes = append(status.CsiNodes, k.String())
		}
		for k := range api.controllers {
			status.Controllers = append(status.Controllers, k.String())
		}
		for k, v := range api.csiNodeIndex {
			status.Nodeindex[k] = types.NamespacedName{
				Namespace: v.Namespace,
				Name:      v.Name,
			}
		}
		status.SysIndexes = api.sysIndexes.debug()
		api.componentsLock.RUnlock()
		api.appPodsLock.RLock()
		for k := range api.appPods {
			status.AppPods = append(status.AppPods, k)
		}
		status.AppIndexes = api.appIndexes.debug()
		api.appPodsLock.RUnlock()
		api.eventsLock.RLock()
		for k, v := range api.events {
			status.Events[k.String()] = len(v)
		}
		api.eventsLock.RUnlock()
		api.pvsLock.RLock()
		for k, v := range api.pvs {
			status.Pvs[k.String()] = types.NamespacedName{
				Namespace: v.Namespace,
				Name:      v.Name,
			}
		}
		status.PvIndexes = api.pvIndexes.debug()
		status.PvcIndexes = api.pvcIndexes.debug()
		api.pvsLock.RUnlock()
		c.IndentedJSON(200, status)
	}
}

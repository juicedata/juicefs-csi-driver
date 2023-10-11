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
	AppPods      []types.NamespacedName
	Nodeindex    map[string]types.NamespacedName
	Events       map[types.NamespacedName]int
	Pvs          map[string]types.NamespacedName
}

func (api *API) debugAPIStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		status := &APIStatus{
			SysNamespace: api.sysNamespace,
			Nodeindex:    make(map[string]types.NamespacedName),
			Events:       make(map[types.NamespacedName]int),
			Pvs:          make(map[string]types.NamespacedName),
		}
		api.componentsLock.RLock()
		for k := range api.mountPods {
			status.MountPods = append(status.MountPods, k)
		}
		for k := range api.csiNodes {
			status.CsiNodes = append(status.CsiNodes, k)
		}
		for k := range api.controllers {
			status.Controllers = append(status.Controllers, k)
		}
		for k, v := range api.nodeindex {
			status.Nodeindex[k] = types.NamespacedName{
				Namespace: v.Namespace,
				Name:      v.Name,
			}
		}
		api.componentsLock.RUnlock()
		api.appPodsLock.RLock()
		for k := range api.appPods {
			status.AppPods = append(status.AppPods, k)
		}
		api.appPodsLock.RUnlock()
		api.eventsLock.RLock()
		for k, v := range api.events {
			status.Events[k] = len(v)
		}
		api.eventsLock.RUnlock()
		api.pvsLock.RLock()
		for k, v := range api.pvs {
			status.Pvs[k] = types.NamespacedName{
				Namespace: v.Namespace,
				Name:      v.Name,
			}
		}
		api.pvsLock.RUnlock()
		c.IndentedJSON(200, status)
	}
}

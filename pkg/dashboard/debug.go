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
	SysIndexes   []types.NamespacedName
	AppIndexes   []types.NamespacedName
	Nodeindex    map[string]types.NamespacedName
	Events       map[string]int
	PvIndexes    []types.NamespacedName
	PvcIndexes   []types.NamespacedName
	Pairs        map[string]types.NamespacedName
}

func (api *API) debugAPIStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		status := &APIStatus{
			SysNamespace: api.sysNamespace,
			Nodeindex:    make(map[string]types.NamespacedName),
			Events:       make(map[string]int),
			Pairs:        make(map[string]types.NamespacedName),
		}
		api.csiNodeLock.RLock()
		for k, v := range api.csiNodeIndex {
			status.Nodeindex[k] = types.NamespacedName{
				Namespace: v.Namespace,
				Name:      v.Name,
			}
		}
		api.csiNodeLock.RUnlock()
		status.SysIndexes = api.sysIndexes.debug()
		status.AppIndexes = api.appIndexes.debug()
		api.eventsLock.RLock()
		for k, v := range api.events {
			status.Events[k.String()] = len(v)
		}
		api.eventsLock.RUnlock()
		api.pairLock.RLock()
		for k, v := range api.pairs {
			status.Pairs[k.String()] = v
		}
		status.PvIndexes = api.pvIndexes.debug()
		status.PvcIndexes = api.pvcIndexes.debug()
		api.pairLock.RUnlock()
		c.IndentedJSON(200, status)
	}
}

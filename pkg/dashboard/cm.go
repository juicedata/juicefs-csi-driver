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
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (api *API) getCSIConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		cm, err := api.client.CoreV1().ConfigMaps(api.sysNamespace).Get(c, "juicefs-csi-driver-config", metav1.GetOptions{})
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
		if cm.Name != "juicefs-csi-driver-config" {
			c.JSON(400, gin.H{"error": "invalid config map name"})
			return
		}
		_, err := api.client.CoreV1().ConfigMaps(api.sysNamespace).Update(c, &cm, metav1.UpdateOptions{})
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, cm)
	}
}

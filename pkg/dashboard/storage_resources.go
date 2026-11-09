/*
 * Copyright 2026 Juicedata Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dashboard

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func storageResource(kind string) client.Object {
	switch kind {
	case "pv":
		return &corev1.PersistentVolume{}
	case "pvc":
		return &corev1.PersistentVolumeClaim{}
	case "storageclass":
		return &storagev1.StorageClass{}
	case "secret":
		return &corev1.Secret{}
	}
	panic("invalid resource kind")
}

func isJuiceSecret(secret *corev1.Secret) bool {
	candidate := secret.DeepCopy()
	if candidate.Data == nil {
		candidate.Data = make(map[string][]byte)
	}
	for key, value := range candidate.StringData {
		candidate.Data[key] = []byte(value)
	}
	return utils.IsJuiceSecret(candidate) || utils.IsJuiceCustSecret(candidate)
}

func validStorageResource(object client.Object) bool {
	switch value := object.(type) {
	case *corev1.PersistentVolume:
		return value.Spec.CSI != nil && value.Spec.CSI.Driver == config.DriverName
	case *storagev1.StorageClass:
		return value.Provisioner == config.DriverName
	case *corev1.Secret:
		return isJuiceSecret(value)
	}
	return true
}

func storageError(c *gin.Context, err error) {
	code := http.StatusInternalServerError
	if status, ok := err.(apierrors.APIStatus); ok {
		code = int(status.Status().Code)
	}
	c.JSON(code, gin.H{"error": err.Error()})
}

func (api *API) getStorageDriver() gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"name": config.DriverName}) }
}

func (api *API) listSCsBasic() gin.HandlerFunc {
	return func(c *gin.Context) {
		classes := storagev1.StorageClassList{}
		if err := api.cachedReader.List(c, &classes); err != nil {
			storageError(c, err)
			return
		}
		names := make([]string, 0, len(classes.Items))
		for _, class := range classes.Items {
			if class.Provisioner == config.DriverName {
				names = append(names, class.Name)
			}
		}
		sort.Strings(names)
		c.JSON(http.StatusOK, names)
	}
}

func (api *API) createStorageResource(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		object := storageResource(kind)
		if err := c.ShouldBindJSON(object); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !validStorageResource(object) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "resource must belong to JuiceFS"})
			return
		}
		if err := api.mgrClient.Create(c, object); err != nil {
			storageError(c, err)
			return
		}
		c.JSON(http.StatusOK, object)
	}
}

func (api *API) updateStorageResource(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current := storageResource(kind)
		key := types.NamespacedName{Namespace: c.Param("namespace"), Name: c.Param("name")}
		if err := api.mgrClient.Get(c, key, current); err != nil {
			storageError(c, err)
			return
		}
		if !validStorageResource(current) {
			c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
			return
		}
		object := storageResource(kind)
		if err := c.ShouldBindJSON(object); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if object.GetName() != key.Name || object.GetNamespace() != key.Namespace {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and namespace cannot be changed"})
			return
		}
		if !validStorageResource(object) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "resource must belong to JuiceFS"})
			return
		}
		if err := api.mgrClient.Update(c, object); err != nil {
			storageError(c, err)
			return
		}
		c.JSON(http.StatusOK, object)
	}
}

func (api *API) deleteStorageResource(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		object := storageResource(kind)
		key := types.NamespacedName{Namespace: c.Param("namespace"), Name: c.Param("name")}
		if err := api.mgrClient.Get(c, key, object); err != nil {
			storageError(c, err)
			return
		}
		if !validStorageResource(object) {
			c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
			return
		}
		if err := api.mgrClient.Delete(c, object); err != nil {
			storageError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "succeed"})
	}
}

type secretSummary struct {
	Metadata metav1.ObjectMeta `json:"metadata"`
	Type     corev1.SecretType `json:"type"`
	Keys     []string          `json:"keys"`
}

func (api *API) listSecretNames() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Query("namespace")
		if namespace == "" {
			c.JSON(http.StatusOK, []string{})
			return
		}
		secrets := &corev1.SecretList{}
		if err := api.cachedReader.List(c, secrets, client.InNamespace(namespace)); err != nil {
			storageError(c, err)
			return
		}
		names := make([]string, 0)
		for _, secret := range secrets.Items {
			if !utils.IsJuiceSecret(&secret) {
				names = append(names, secret.Name)
			}
		}
		sort.Strings(names)
		c.JSON(http.StatusOK, names)
	}
}

func (api *API) listJuiceSecrets() gin.HandlerFunc {
	return func(c *gin.Context) {
		secrets, err := api.secretSvc.ListAllSecrets(c)
		if err != nil {
			storageError(c, err)
			return
		}
		result := make([]secretSummary, 0, len(secrets))
		for _, secret := range secrets {
			if !isJuiceSecret(&secret) {
				continue
			}
			keys := make([]string, 0, len(secret.Data))
			for key := range secret.Data {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			result = append(result, secretSummary{Metadata: secret.ObjectMeta, Type: secret.Type, Keys: keys})
		}
		c.JSON(http.StatusOK, result)
	}
}

func (api *API) getJuiceSecret() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := &corev1.Secret{}
		if err := api.cachedReader.Get(c, types.NamespacedName{Namespace: c.Param("namespace"), Name: c.Param("name")}, secret); err != nil {
			storageError(c, err)
			return
		}
		if !isJuiceSecret(secret) {
			c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
			return
		}
		c.JSON(http.StatusOK, secret)
	}
}

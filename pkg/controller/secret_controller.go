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

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var (
	secretCtrlLog                   = klog.NewKlogr().WithName("secret-controller")
	secretLastUpdateAtAnnotationKey = "juicefs/last-update-at"
	secretFieldsHashAnnotationKey   = "juicefs/secret-fields-hash"
)

type SecretController struct {
	*k8sclient.K8sClient
}

func NewSecretController(client *k8sclient.K8sClient) *SecretController {
	return &SecretController{client}
}

func checkAndCleanOrphanSecret(ctx context.Context, client *k8sclient.K8sClient, secrets *corev1.Secret) error {
	if secrets.Namespace != config.Namespace {
		return nil
	}
	// new version of juicefs-csi-driver has a label to identify the secret
	// no need to manual clean up
	if _, ok := secrets.Labels[common.JuicefsSecretLabelKey]; ok {
		return nil
	}
	if !strings.HasPrefix(secrets.Name, "juicefs-") || !strings.HasSuffix(secrets.Name, "-secret") {
		return nil
	}
	if secrets.Data["token"] == nil && secrets.Data["metaurl"] == nil {
		return nil
	}
	// the secret is created less than an hour, clean later
	if !time.Now().After(secrets.CreationTimestamp.Add(time.Hour)) {
		return nil
	}
	// check if the secret is mount pod's secret
	if secrets.Data["check_mount.sh"] == nil {
		return nil
	}

	// check if the pod still exists
	podName := strings.TrimSuffix(secrets.Name, "-secret")
	if _, err := client.GetPod(ctx, podName, secrets.Namespace); k8serrors.IsNotFound(err) {
		secretCtrlLog.Info("orphan secret found, delete it", "name", secrets.Name)
		if err := client.DeleteSecret(ctx, secrets.Name, secrets.Namespace); err != nil {
			secretCtrlLog.Error(err, "delete secret error", "name", secrets.Name)
			return err
		}
		return nil
	}

	return nil
}

func refreshSecretInitConfig(ctx context.Context, client *k8sclient.K8sClient, name, namespace string) error {
	secretCtrlLog.V(1).Info("refresh secret initconfig", "namespace", namespace, "name", name)
	secrets, err := client.GetSecret(ctx, name, namespace)
	if err != nil {
		secretCtrlLog.Error(err, "get secret error", "namespace", namespace, "name", name)
		return ctrlclient.IgnoreNotFound(err)
	}

	if err := checkAndCleanOrphanSecret(ctx, client, secrets); err != nil {
		secretCtrlLog.Error(err, "check and clean orphan secret error", "namespace", namespace, "name", name)
		return err
	}

	if metaurl, found := secrets.Data["metaurl"]; found && len(metaurl) > 0 {
		secretCtrlLog.V(1).Info("metaurl found in secret, ce volume, ignore", "namespace", namespace, "name", name)
		return nil
	}

	if token, found := secrets.Data["token"]; !found || len(token) == 0 {
		secretCtrlLog.V(1).Info("token not found in secret", "namespace", namespace, "name", name)
		return nil
	}
	if _, found := secrets.Data["name"]; !found {
		secretCtrlLog.V(1).Info("name not found in secret", "namespace", namespace, "name", name)
		return nil
	}

	jfs := juicefs.NewJfsProvider(nil, client)
	secretsMap := make(map[string]string)
	for k, v := range secrets.Data {
		secretsMap[k] = string(v[:])
	}
	jfsSetting, err := jfs.Settings(ctx, "", "", "", secretsMap, nil, nil)
	if err != nil {
		return err
	}
	if jfsSetting.IsCe {
		secretCtrlLog.V(1).Info("ce volume, no need to refresh initconfig", "namespace", namespace, "name", name)
		return nil
	}
	hashSecretsMap := make(map[string]string)
	maps.Copy(hashSecretsMap, secretsMap)
	config.KeysCompatible(hashSecretsMap)
	hashFields := []string{"token", "name", "access-key", "secret-key", "access-key2", "secret-key2", "bucket", "envs"}
	var hashParts []string
	for _, field := range hashFields {
		if v, ok := hashSecretsMap[field]; ok {
			hashParts = append(hashParts, field+"="+v)
		}
	}
	sort.Strings(hashParts)
	h := sha256.Sum256([]byte(strings.Join(hashParts, ";")))
	currentHash := hex.EncodeToString(h[:])

	storedHash := ""
	if secrets.Annotations != nil {
		storedHash = secrets.Annotations[secretFieldsHashAnnotationKey]
	}
	forceUpdate := currentHash != storedHash
	if _, ok := secretsMap["initconfig"]; ok && !forceUpdate {
		if v, ok := secrets.Annotations[secretLastUpdateAtAnnotationKey]; ok {
			t, err := time.Parse(time.RFC3339, v)
			if err == nil && time.Since(t) < config.SecretReconcilerInterval {
				secretCtrlLog.V(1).Info("initconfig updated too frequently, skip", "namespace", namespace, "name", name)
				return nil
			}
		}
	}

	tempConfDir, err := os.MkdirTemp(os.TempDir(), "juicefs-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempConfDir)

	initconfigs := ""
	jfsSetting.ClientConfPath = tempConfDir
	output, err := jfs.AuthFs(ctx, secretsMap, jfsSetting, true)
	if err != nil {
		secretCtrlLog.Error(err, "auth failed", "output", output)
		initconfigs = ""
	} else {
		conf := jfsSetting.Name + ".conf"
		confPath := filepath.Join(jfsSetting.ClientConfPath, conf)
		b, err := os.ReadFile(confPath)
		if err == nil {
			initconfigs = string(b)
		}
	}

	if initconfigs != "" {
		secretsMap["initconfig"] = initconfigs
		if secrets.Annotations == nil {
			secrets.Annotations = make(map[string]string)
		}
		secrets.Annotations[secretLastUpdateAtAnnotationKey] = time.Now().Format(time.RFC3339)
		secrets.Annotations[secretFieldsHashAnnotationKey] = currentHash
	} else {
		// if force update and auth failed, remove the initconfig
		if forceUpdate {
			delete(secretsMap, "initconfig")
			delete(secrets.Data, "initconfig")
		}
	}

	secrets.StringData = secretsMap
	return client.UpdateSecret(ctx, secrets)
}

func (m *SecretController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	name := request.Name
	namespace := request.Namespace
	secretCtrlLog.V(1).Info("Receive secret", "namespace", namespace, "name", name)
	secrets, err := m.GetSecret(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		secretCtrlLog.Error(err, "get secret error", "namespace", namespace, "name", name)
		return reconcile.Result{}, err
	}
	if secrets == nil {
		secretCtrlLog.V(1).Info("secret has been deleted.", "namespace", namespace, "name", name)
		return reconcile.Result{}, nil
	}

	if err := checkAndCleanOrphanSecret(ctx, m.K8sClient, secrets); err != nil {
		secretCtrlLog.Error(err, "check and clean orphan secret error", "namespace", namespace, "name", name)
		return reconcile.Result{}, err
	}

	if err := refreshSecretInitConfig(ctx, m.K8sClient, request.Name, request.Namespace); err != nil {
		secretCtrlLog.Error(err, "refresh secret initconfig error", "namespace", namespace, "name", name)
		return reconcile.Result{}, err
	}
	// requeue after to make sure the initconfig is always up-to-date
	return reconcile.Result{Requeue: true, RequeueAfter: config.SecretReconcilerInterval}, nil
}

func shouldSecretInQueue(secret *corev1.Secret) bool {
	if _, ok := watchedSecrets[fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)]; ok {
		return true
	}
	return false
}

func (m *SecretController) SetupWithManager(mgr ctrl.Manager) error {
	secretCtrlLog.V(1).Info("SetupWithManager", "name", "secret-controller")
	c, err := controller.New("secret", mgr, controller.Options{Reconciler: m})
	if err != nil {
		return err
	}

	return c.Watch(source.Kind(mgr.GetCache(), &corev1.Secret{}, &handler.TypedEnqueueRequestForObject[*corev1.Secret]{}, predicate.TypedFuncs[*corev1.Secret]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.Secret]) bool {
			secret := event.Object
			secretCtrlLog.V(1).Info("watch secret created", "name", secret.GetName())
			return shouldSecretInQueue(secret)
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.Secret]) bool {
			secretNew, secretOld := updateEvent.ObjectNew, updateEvent.ObjectOld
			if secretNew.GetResourceVersion() == secretOld.GetResourceVersion() {
				secretCtrlLog.V(1).Info("secret.onUpdateFunc Skip due to resourceVersion not changed")
				return false
			}
			return shouldSecretInQueue(secretNew)
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.Secret]) bool {
			secret := deleteEvent.Object
			secretCtrlLog.V(1).Info("watch secret deleted", "name", secret.GetName())
			return shouldSecretInQueue(secret)
		},
	}))
}

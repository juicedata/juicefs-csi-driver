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
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
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
	secretCtrlLog = klog.NewKlogr().WithName("secret-controller")
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

func (m *SecretController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	secretCtrlLog.V(1).Info("Receive secret", "name", request.Name, "namespace", request.Namespace)
	secrets, err := m.GetSecret(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		secretCtrlLog.Error(err, "get secret error", "name", request.Name)
		return reconcile.Result{}, err
	}
	if secrets == nil {
		secretCtrlLog.V(1).Info("secret has been deleted.", "name", request.Name)
		return reconcile.Result{}, nil
	}

	if err := checkAndCleanOrphanSecret(ctx, m.K8sClient, secrets); err != nil {
		secretCtrlLog.Error(err, "check and clean orphan secret error", "name", request.Name)
		return reconcile.Result{}, err
	}

	if _, found := secrets.Data["token"]; !found {
		secretCtrlLog.V(1).Info("token not found in secret", "name", request.Name)
		return reconcile.Result{}, nil
	}
	if _, found := secrets.Data["name"]; !found {
		secretCtrlLog.V(1).Info("name not found in secret", "name", request.Name)
		return reconcile.Result{}, nil
	}
	jfs := juicefs.NewJfsProvider(nil, nil)
	secretsMap := make(map[string]string)
	for k, v := range secrets.Data {
		secretsMap[k] = string(v[:])
	}
	jfsSetting, err := jfs.Settings(ctx, "", secretsMap, nil, nil)
	if err != nil {
		return reconcile.Result{}, err
	}
	tempConfDir, err := os.MkdirTemp(os.TempDir(), "juicefs-")
	if err != nil {
		return reconcile.Result{}, err
	}
	defer os.RemoveAll(tempConfDir)
	jfsSetting.ClientConfPath = tempConfDir
	output, err := jfs.AuthFs(ctx, secretsMap, jfsSetting, true)
	if err != nil {
		secretCtrlLog.Error(err, "auth failed", "output", output)
		return reconcile.Result{}, err
	}
	conf := jfsSetting.Name + ".conf"
	confPath := filepath.Join(jfsSetting.ClientConfPath, conf)
	b, err := os.ReadFile(confPath)
	if err != nil {
		secretCtrlLog.Error(err, "read initconfig failed", "conf", conf)
		return reconcile.Result{}, err
	}
	confs := string(b)
	secretsMap["initconfig"] = confs
	secrets.StringData = secretsMap
	err = m.UpdateSecret(ctx, secrets)
	if err != nil {
		secretCtrlLog.Error(err, "inject initconfig failed", "name", request.Name)
		return reconcile.Result{}, err
	}
	// requeue after to make sure the initconfig is always up-to-date
	return reconcile.Result{Requeue: true, RequeueAfter: config.SecretReconcilerInterval}, nil
}

func (m *SecretController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("secret", mgr, controller.Options{Reconciler: m})
	if err != nil {
		return err
	}

	return c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			secret := event.Object.(*corev1.Secret)
			secretCtrlLog.V(1).Info("watch secret created", "name", secret.GetName())
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			secretNew, ok := updateEvent.ObjectNew.(*corev1.Secret)
			if !ok {
				secretCtrlLog.V(1).Info("secret.onUpdateFunc Skip object", "object", updateEvent.ObjectNew)
				return false
			}
			secretCtrlLog.V(1).Info("watch secret updated", "name", secretNew.GetName())

			secretOld, ok := updateEvent.ObjectOld.(*corev1.Secret)
			if !ok {
				secretCtrlLog.V(1).Info("secret.onUpdateFunc Skip object", "object", updateEvent.ObjectOld)
				return false
			}

			if secretNew.GetResourceVersion() == secretOld.GetResourceVersion() {
				secretCtrlLog.V(1).Info("secret.onUpdateFunc Skip due to resourceVersion not changed")
				return false
			}

			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			secret := deleteEvent.Object.(*corev1.Secret)
			secretCtrlLog.V(1).Info("watch secret deleted", "name", secret.GetName())
			return false
		},
	})
}

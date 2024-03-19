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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type SecretController struct {
	*k8sclient.K8sClient
}

func NewSecretController(client *k8sclient.K8sClient) *SecretController {
	return &SecretController{client}
}

func (m *SecretController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(6).Infof("Receive secret %s %s", request.Name, request.Namespace)
	secrets, err := m.GetSecret(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		klog.Errorf("get secret %s error: %v", request.Name, err)
		return reconcile.Result{}, err
	}
	if secrets == nil {
		klog.V(6).Infof("secret %s has been deleted.", request.Name)
		return reconcile.Result{}, nil
	}
	if _, found := secrets.Data["token"]; !found {
		klog.V(6).Infof("token not found in secret %s", request.Name)
		return reconcile.Result{}, nil
	}
	if _, found := secrets.Data["name"]; !found {
		klog.V(6).Infof("name not found in secret %s", request.Name)
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
		klog.Errorf("auth failed: %s, %v", output, err)
		return reconcile.Result{}, err
	}
	conf := jfsSetting.Name + ".conf"
	confPath := filepath.Join(jfsSetting.ClientConfPath, conf)
	b, err := os.ReadFile(confPath)
	if err != nil {
		klog.Errorf("read initconfig %s failed: %v", conf, err)
		return reconcile.Result{}, err
	}
	confs := string(b)
	secretsMap["initconfig"] = confs
	secrets.StringData = secretsMap
	err = m.UpdateSecret(ctx, secrets)
	if err != nil {
		klog.Errorf("inject initconfig into %s failed: %v", request.Name, err)
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
			klog.V(6).Infof("watch secret %s created", secret.GetName())
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			secretNew, ok := updateEvent.ObjectNew.(*corev1.Secret)
			if !ok {
				klog.V(6).Infof("secret.onUpdateFunc Skip object: %v", updateEvent.ObjectNew)
				return false
			}
			klog.V(6).Infof("watch secret %s updated", secretNew.GetName())

			secretOld, ok := updateEvent.ObjectOld.(*corev1.Secret)
			if !ok {
				klog.V(6).Infof("secret.onUpdateFunc Skip object: %v", updateEvent.ObjectOld)
				return false
			}

			if secretNew.GetResourceVersion() == secretOld.GetResourceVersion() {
				klog.V(6).Info("secret.onUpdateFunc Skip due to resourceVersion not changed")
				return false
			}

			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			secret := deleteEvent.Object.(*corev1.Secret)
			klog.V(6).Infof("watch secret %s deleted", secret.GetName())
			return false
		},
	})
}

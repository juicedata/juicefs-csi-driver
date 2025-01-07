// Copyright 2025 Juicedata Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secrets

import (
	"context"

	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	secretLog = klog.NewKlogr().WithName("SecretService/Cache")
)

type CacheSecretService struct {
	*secretService

	secretIndexes *utils.TimeOrderedIndexes[corev1.Secret]
}

func (c *CacheSecretService) ListAllSecrets(ctx context.Context) ([]corev1.Secret, error) {
	secrets := make([]corev1.Secret, 0)
	for name := range c.secretIndexes.Iterate(ctx, false) {
		var secret corev1.Secret
		if err := c.client.Get(ctx, name, &secret); err == nil {
			secrets = append(secrets, secret)
		}
	}
	return secrets, nil
}

func (c *CacheSecretService) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	secret := &corev1.Secret{}
	if err := c.client.Get(ctx, req.NamespacedName, secret); err != nil {
		secretLog.Error(err, "get secret failed", "namespacedName", req.NamespacedName)
		return reconcile.Result{}, nil
	}
	if secret.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}
	if utils.IsJuiceSecret(secret) || utils.IsJuiceCustSecret(secret) {
		c.secretIndexes.AddIndex(
			secret,
			func(p *corev1.Secret) metav1.ObjectMeta { return p.ObjectMeta },
			func(name types.NamespacedName) (*corev1.Secret, error) {
				var s corev1.Secret
				err := c.client.Get(ctx, name, &s)
				return &s, err
			},
		)
	}
	secretLog.V(1).Info("secret created", "namespacedName", req.NamespacedName)
	return reconcile.Result{}, nil
}

func (c *CacheSecretService) SetupWithManager(mgr manager.Manager) error {
	ctr, err := controller.New("secret", mgr, controller.Options{Reconciler: c})
	if err != nil {
		return err
	}

	return ctr.Watch(source.Kind(mgr.GetCache(), &corev1.Secret{}, &handler.TypedEnqueueRequestForObject[*corev1.Secret]{}, predicate.TypedFuncs[*corev1.Secret]{
		CreateFunc: func(event event.TypedCreateEvent[*corev1.Secret]) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*corev1.Secret]) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.TypedDeleteEvent[*corev1.Secret]) bool {
			secret := deleteEvent.Object
			indexes := c.secretIndexes
			if indexes != nil && (utils.IsJuiceSecret(secret) || utils.IsJuiceCustSecret(secret)) {
				indexes.RemoveIndex(types.NamespacedName{
					Namespace: secret.GetNamespace(),
					Name:      secret.GetName(),
				})
				secretLog.V(1).Info("secret deleted", "namespace", secret.GetNamespace(), "name", secret.GetName())
				return false
			}
			return true
		},
	}))
}

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

package resource

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func CreateOrUpdateSecret(ctx context.Context, client *k8sclient.K8sClient, secret *corev1.Secret) error {
	log := util.GenLog(ctx, log, "createOrUpdateSecret")
	log.Info("secret", "name", secret.Name, "namespace", secret.Namespace)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		oldSecret, err := client.GetSecret(ctx, secret.Name, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// secret not exist, create
				_, err := client.CreateSecret(ctx, secret)
				return err
			}
			// unexpected err
			return err
		}
		shouldUpdate := false
		for k, v := range secret.Data {
			if oldSecret.Data[k] == nil {
				shouldUpdate = true
				break
			}
			if string(oldSecret.Data[k]) != string(v) {
				shouldUpdate = true
				break
			}
		}

		oldSecret.StringData = secret.StringData
		// merge owner reference
		if len(secret.OwnerReferences) != 0 {
			newOwner := secret.OwnerReferences[0]
			exist := false
			for _, ref := range oldSecret.OwnerReferences {
				if ref.UID == newOwner.UID {
					exist = true
					break
				}
			}
			if !exist {
				shouldUpdate = true
				oldSecret.OwnerReferences = append(oldSecret.OwnerReferences, newOwner)
			}
		}
		if !shouldUpdate {
			log.V(1).Info("secret not changed, skip update", "name", secret.Name)
			return nil
		}
		return client.UpdateSecret(ctx, oldSecret)
	})
	if err != nil {
		log.Error(err, "create or update secret error", "secretName", secret.Name)
		return err
	}
	return nil
}

func GetSecretNameByUniqueId(uniqueId string) string {
	return fmt.Sprintf("juicefs-%s-secret", uniqueId)
}

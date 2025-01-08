/*
 Copyright 2025 Juicedata Inc

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

package secrets

import (
	"context"

	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type secretService struct {
	client client.Client
}

func (c *secretService) ListAllSecrets(ctx context.Context) ([]corev1.Secret, error) {
	seccretList := &corev1.SecretList{}
	if err := c.client.List(ctx, seccretList); err != nil {
		return nil, err
	}
	result := make([]corev1.Secret, 0, len(seccretList.Items))
	for _, secret := range seccretList.Items {
		if utils.IsJuiceSecret(&secret) || utils.IsJuiceCustSecret(&secret) {
			result = append(result, secret)
		}
	}
	return result, nil
}

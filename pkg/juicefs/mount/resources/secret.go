/*
Copyright 2022 Juicedata Inc

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

package resources

import (
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func NewSecret(setting *config.JfsSetting, generateName string) corev1.Secret {
	data := make(map[string]string)
	if setting.MetaUrl != "" {
		data["metaurl"] = setting.MetaUrl
	}
	if setting.SecretKey != "" {
		data["secretkey"] = setting.SecretKey
	}
	if setting.SecretKey2 != "" {
		data["secretkey2"] = setting.SecretKey2
	}
	if setting.Token != "" {
		data["token"] = setting.Token
	}
	if setting.Passphrase != "" {
		data["passphrase"] = setting.Passphrase
	}
	if setting.EncryptRsaKey != "" {
		data["encrypt_rsa_key"] = setting.EncryptRsaKey
	}
	if setting.InitConfig != "" {
		data["init_config"] = setting.InitConfig
	}
	for k, v := range setting.Envs {
		data[k] = v
	}
	klog.V(6).Infof("secret data: %v", data)
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    config.Namespace,
			GenerateName: generateName,
		},
		StringData: data,
	}
	return secret
}

func SetPodAsOwner(secret *corev1.Secret, owner corev1.Pod) {
	controller := true
	secret.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: owner.APIVersion,
		Kind:       owner.Kind,
		Name:       owner.Name,
		UID:        owner.UID,
		Controller: &controller,
	}})
}

func SetJobAsOwner(secret *corev1.Secret, owner batchv1.Job) {
	controller := true
	secret.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: owner.APIVersion,
		Kind:       owner.Kind,
		Name:       owner.Name,
		UID:        owner.UID,
		Controller: &controller,
	}})
}

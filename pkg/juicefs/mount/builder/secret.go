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

package builder

import (
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	checkMountScriptName = "check_mount.sh"
	checkMountScriptPath = "/" + checkMountScriptName
)

var (
	checkMountScriptContent = `ConditionPathIsMountPoint="$1"
subpath="$2"
count=0
while ! mount | grep $ConditionPathIsMountPoint | grep JuiceFS
do
    sleep 3
    count=¬expr $count + 1¬
    if test $count -eq 10
    then
        echo "timed out!"
        exit 1
    fi
done
echo "$(date "+%Y-%m-%d %H:%M:%S")"
echo "succeed in checking mount point $ConditionPathIsMountPoint"
if [ -n "$subpath" ]; then
echo "create subpath $subpath"
mkdir -r 777 $ConditionPathIsMountPoint/$subpath
fi;
`
)

func (r *BaseBuilder) NewSecret() corev1.Secret {
	data := make(map[string]string)
	if r.jfsSetting.MetaUrl != "" {
		data["metaurl"] = r.jfsSetting.MetaUrl
	}
	if r.jfsSetting.SecretKey != "" {
		data["secretkey"] = r.jfsSetting.SecretKey
	}
	if r.jfsSetting.SecretKey2 != "" {
		data["secretkey2"] = r.jfsSetting.SecretKey2
	}
	if r.jfsSetting.Token != "" {
		data["token"] = r.jfsSetting.Token
	}
	if r.jfsSetting.Passphrase != "" {
		data["passphrase"] = r.jfsSetting.Passphrase
	}
	if r.jfsSetting.EncryptRsaKey != "" {
		data["encrypt_rsa_key"] = r.jfsSetting.EncryptRsaKey
	}
	if r.jfsSetting.InitConfig != "" {
		data["init_config"] = r.jfsSetting.InitConfig
	}
	replacer := strings.NewReplacer("¬", "`")
	data[checkMountScriptName] = replacer.Replace(checkMountScriptContent)
	if options, err := r.jfsSetting.ParseFormatOptions(); err == nil {
		for _, pair := range options {
			if pair[0] == "session-token" {
				data["session-token"] = pair[1]
			}
		}
	}
	for k, v := range r.jfsSetting.Envs {
		data[k] = v
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.jfsSetting.Attr.Namespace,
			Name:      r.jfsSetting.SecretName,
		},
		StringData: data,
	}
	return secret
}

func SetPodAsOwner(secret *corev1.Secret, owner corev1.Pod) {
	controller := true
	secret.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       owner.Name,
		UID:        owner.UID,
		Controller: &controller,
	}})
}

func SetPVCAsOwner(secret *corev1.Secret, owner *corev1.PersistentVolumeClaim) {
	controller := true
	secret.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Name:       owner.Name,
		UID:        owner.UID,
		Controller: &controller,
	}})
}

func SetJobAsOwner(secret *corev1.Secret, owner batchv1.Job) {
	controller := true
	secret.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       owner.Name,
		UID:        owner.UID,
		Controller: &controller,
	}})
}

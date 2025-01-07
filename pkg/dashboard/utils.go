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

package dashboard

import (
	"os"
	"regexp"
	"sort"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
)

type ReverseSort struct {
	sort.Interface
}

func (r *ReverseSort) Less(i, j int) bool {
	return !r.Interface.Less(i, j)
}

func Reverse(data sort.Interface) sort.Interface {
	return &ReverseSort{data}
}

func LabelSelectorOfMount(pv corev1.PersistentVolume) labels.Selector {
	values := []string{pv.Spec.CSI.VolumeHandle}
	if pv.Spec.StorageClassName != "" {
		values = append(values, pv.Spec.StorageClassName)
	}
	sl := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      common.PodUniqueIdLabelKey,
			Operator: metav1.LabelSelectorOpIn,
			Values:   values,
		}},
	}
	labelMap, _ := metav1.LabelSelectorAsSelector(&sl)
	return labelMap
}

func getSysNamespace() string {
	namespace := "kube-system"
	if os.Getenv("SYS_NAMESPACE") != "" {
		namespace = os.Getenv("SYS_NAMESPACE")
	}
	return namespace
}

func isShareMount(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "STORAGE_CLASS_SHARE_MOUNT" && env.Value == "true" {
			return true
		}
	}

	return false
}

func SetJobAsConfigMapOwner(cm *corev1.ConfigMap, owner *batchv1.Job) {
	controller := true
	cm.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       owner.Name,
		UID:        owner.UID,
		Controller: &controller,
	}})
}

func getUniqueIdFromSecretName(secretName string) string {
	re := regexp.MustCompile(`juicefs-(.*?)-secret`)
	match := re.FindStringSubmatch(secretName)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

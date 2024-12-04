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

package config

import (
	"context"
	"encoding/json"
	"os"
	"sort"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

const UpgradeConfigMapName = "juicefs-upgrade-batch"

type BatchConfig struct {
	Parallel    int                 `json:"parallel"`
	IgnoreError bool                `json:"ignoreError"`
	NoRecreate  bool                `json:"norecreate,omitempty"`
	Node        string              `json:"node,omitempty"`
	UniqueId    string              `json:"uniqueId,omitempty"`
	Batches     [][]MountPodUpgrade `json:"batches"`
	Status      UpgradeStatus       `json:"status"`
}

type MountPodUpgrade struct {
	Name       string        `json:"name"`
	Node       string        `json:"node"`
	CSINodePod string        `json:"csiNodePod"`
	Status     UpgradeStatus `json:"status"`
}

type UpgradeStatus string

const (
	Pending UpgradeStatus = "pending"
	Running UpgradeStatus = "running"
	Success UpgradeStatus = "success"
	Fail    UpgradeStatus = "fail"
)

func NewBatchConfig(pods []corev1.Pod, parallel int, ignoreError bool, recreate bool, nodeName string, uniqueId string, csiNodes []corev1.Pod) *BatchConfig {
	batchConf := &BatchConfig{
		Parallel:    parallel,
		IgnoreError: ignoreError,
		NoRecreate:  !recreate,
		Node:        nodeName,
		UniqueId:    uniqueId,
	}

	csiNodesMap := make(map[string]corev1.Pod)
	for _, csi := range csiNodes {
		csiNodesMap[csi.Spec.NodeName] = csi
	}

	sort.Sort(podList(pods))

	index := 0
	j := 0
	batches := make([][]MountPodUpgrade, (len(pods)+parallel-1)/parallel)
	for _, pod := range pods {
		mountPod := MountPodUpgrade{
			Name:       pod.Name,
			Node:       pod.Spec.NodeName,
			CSINodePod: csiNodesMap[pod.Spec.NodeName].Name,
		}
		batches[j] = append(batches[j], mountPod)
		index += 1

		if index == parallel {
			j += 1
			index = 0
		}
	}
	batchConf.Batches = batches
	return batchConf
}

type podList []corev1.Pod

func (p podList) Len() int {
	return len(p)
}

func (p podList) Less(i, j int) bool {
	if p[i].Spec.NodeName < p[j].Spec.NodeName {
		return true
	}
	if p[i].Spec.NodeName > p[j].Spec.NodeName {
		return false
	}
	return p[i].Annotations[common.UniqueId] < p[j].Annotations[common.UniqueId]
}

func (p podList) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func LoadUpgradeConfig(ctx context.Context, client *k8s.K8sClient, configName string) (*BatchConfig, error) {
	sysNamespace := os.Getenv("SYS_NAMESPACE")
	if sysNamespace == "" {
		sysNamespace = "kube-system"
	}
	if configName == "" {
		configName = UpgradeConfigMapName
	}
	cm, err := client.GetConfigMap(ctx, configName, sysNamespace)
	if err != nil {
		return nil, err
	}

	cfg := &BatchConfig{}

	err = json.Unmarshal([]byte(cm.Data["upgrade"]), cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func SaveUpgradeConfig(ctx context.Context, client *k8s.K8sClient, configName string, config *BatchConfig) (*corev1.ConfigMap, error) {
	sysNamespace := os.Getenv("SYS_NAMESPACE")
	if sysNamespace == "" {
		sysNamespace = "kube-system"
	}
	if configName == "" {
		configName = UpgradeConfigMapName
	}
	var cfg *corev1.ConfigMap
	var err error
	if cfg, err = client.GetConfigMap(ctx, configName, sysNamespace); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
		cfg = nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configName,
				Namespace: sysNamespace,
			},
			Data: map[string]string{"upgrade": string(data)},
		}
		return cfg, client.CreateConfigMap(ctx, cfg)

	}
	cfg.Data = map[string]string{"upgrade": string(data)}
	return cfg, client.UpdateConfigMap(ctx, cfg)
}

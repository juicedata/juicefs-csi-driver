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
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

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
	Stop    UpgradeStatus = "stop"
	Pause   UpgradeStatus = "pause"
)

// used by kubectl plugin
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
			Status:     Pending,
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
	if configName == "" {
		return nil, fmt.Errorf("config name is empty")
	}
	cm, err := client.GetConfigMap(ctx, configName, Namespace)
	if err != nil {
		return nil, err
	}

	return LoadBatchConfig(cm)
}

func LoadBatchConfig(cm *corev1.ConfigMap) (*BatchConfig, error) {
	cfg := &BatchConfig{}

	err := json.Unmarshal([]byte(cm.Data["upgrade"]), cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// GetAllUpgradeConfigs retrieves all upgrade configurations from ConfigMaps
func GetAllUpgradeConfigs(ctx context.Context, client *k8s.K8sClient) (map[string]*BatchConfig, error) {
	s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.PodTypeKey: common.ConfigTypeValue,
		},
	})
	cmList, err := client.CoreV1().ConfigMaps(Namespace).List(ctx, metav1.ListOptions{LabelSelector: s.String()})
	if err != nil {
		return nil, err
	}

	configs := make(map[string]*BatchConfig)
	for _, cm := range cmList.Items {
		cfg, err := LoadBatchConfig(&cm)
		if err != nil {
			return nil, err
		}
		configs[cm.Name] = cfg
	}
	return configs, nil
}

func CreateUpgradeConfig(ctx context.Context, client *k8s.K8sClient, configName string, config *BatchConfig) (*corev1.ConfigMap, error) {
	if configName == "" {
		return nil, fmt.Errorf("config name is empty")
	}
	var cfg *corev1.ConfigMap
	var err error
	if cfg, err = client.GetConfigMap(ctx, configName, Namespace); err != nil {
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
				Namespace: Namespace,
				Labels: map[string]string{
					common.PodTypeKey: common.ConfigTypeValue,
				},
			},
			Data: map[string]string{"upgrade": string(data)},
		}
		return cfg, client.CreateConfigMap(ctx, cfg)

	}
	return nil, fmt.Errorf("config %s already exists", configName)
}

func UpdateUpgradeConfig(ctx context.Context, client *k8s.K8sClient, configName string, config *BatchConfig) (*corev1.ConfigMap, error) {
	if configName == "" {
		return nil, fmt.Errorf("config name is empty")
	}
	var cfg *corev1.ConfigMap
	var err error
	if cfg, err = client.GetConfigMap(ctx, configName, Namespace); err != nil {
		return nil, err
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	cfg.Data = map[string]string{"upgrade": string(data)}
	return cfg, client.UpdateConfigMap(ctx, cfg)
}

func GetDiff(mountPod *corev1.Pod, pvc *corev1.PersistentVolumeClaim, pv *corev1.PersistentVolume, secret, custSecret *corev1.Secret) (oldSetting *JfsSetting, newSetting *JfsSetting, err error) {
	return GetDiffWithNode(mountPod, pvc, pv, secret, custSecret, nil)
}

func GetDiffWithNode(mountPod *corev1.Pod, pvc *corev1.PersistentVolumeClaim, pv *corev1.PersistentVolume, secret, custSecret *corev1.Secret, node *corev1.Node) (oldSetting *JfsSetting, newSetting *JfsSetting, err error) {
	oldSetting, err = RevertSettingWithNode(mountPod, pvc, pv, secret, custSecret, node)
	if err != nil {
		return
	}

	newSetting, err = RevertSettingWithNode(mountPod, pvc, pv, secret, custSecret, node)
	if err != nil {
		return
	}

	if err = newSetting.ReNew(mountPod, pvc, pv, custSecret); err != nil {
		return
	}
	newSetting = newSetting.Safe(oldSetting)
	oldSetting = oldSetting.Safe(nil)
	return
}

// FilterPodsNotInOngoingUpgrade filters out pods that are currently in ongoing upgrade tasks.
// It queries all upgrade configurations and returns the filtered pod list and a list of pod names that were skipped.
func FilterPodsNotInOngoingUpgrade(ctx context.Context, client *k8s.K8sClient, pods []corev1.Pod) ([]corev1.Pod, []string, error) {
	if len(pods) == 0 {
		return pods, nil, nil
	}

	// Get all upgrade configurations
	configs, err := GetAllUpgradeConfigs(ctx, client)
	if err != nil {
		return nil, nil, err
	}

	if len(configs) == 0 {
		return pods, nil, nil
	}

	// Collect all pod names that are in ongoing upgrade jobs
	podsInOngoingJobs := make(map[string]struct{})
	for _, cfg := range configs {
		// Only consider Pending or Running or Pause status as "in progress"
		if cfg.Status != Pending && cfg.Status != Running && cfg.Status != Pause {
			continue
		}
		for _, batch := range cfg.Batches {
			for _, pod := range batch {
				if pod.Name != "" && pod.Status != Success && pod.Status != Fail && pod.Status != Stop {
					podsInOngoingJobs[pod.Name] = struct{}{}
				}
			}
		}
	}

	if len(podsInOngoingJobs) == 0 {
		return pods, nil, nil
	}

	// Filter pods and collect skipped pod names
	skippedPods := make([]string, 0)
	filteredPods := make([]corev1.Pod, 0, len(pods))
	for _, pod := range pods {
		if _, exists := podsInOngoingJobs[pod.Name]; exists {
			skippedPods = append(skippedPods, pod.Name)
			continue
		}
		filteredPods = append(filteredPods, pod)
	}

	sort.Strings(skippedPods)
	return filteredPods, skippedPods, nil
}

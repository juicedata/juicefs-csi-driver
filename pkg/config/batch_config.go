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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type BatchConfig struct {
	Parallel    int               `json:"parallel"`
	IgnoreError bool              `json:"ignoreError"`
	NoRecreate  bool              `json:"norecreate,omitempty"`
	Kind        UpgradeKind       `json:"kind,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Node        string            `json:"node,omitempty"`
	UniqueId    string            `json:"uniqueId,omitempty"`
	Batches     [][]UpgradeTarget `json:"batches"`
	Status      UpgradeStatus     `json:"status"`
}

// UnmarshalJSON handles backward compatibility with old MountPodUpgrade format
func (bc *BatchConfig) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*bc = BatchConfig{}

	if v, ok := raw["parallel"]; ok {
		if err := json.Unmarshal(v, &bc.Parallel); err != nil {
			return err
		}
	}
	if v, ok := raw["ignoreError"]; ok {
		if err := json.Unmarshal(v, &bc.IgnoreError); err != nil {
			return err
		}
	}
	if v, ok := raw["norecreate"]; ok {
		if err := json.Unmarshal(v, &bc.NoRecreate); err != nil {
			return err
		}
	}
	if v, ok := raw["kind"]; ok {
		if err := json.Unmarshal(v, &bc.Kind); err != nil {
			return err
		}
	}
	if v, ok := raw["namespace"]; ok {
		if err := json.Unmarshal(v, &bc.Namespace); err != nil {
			return err
		}
	}
	if v, ok := raw["node"]; ok {
		if err := json.Unmarshal(v, &bc.Node); err != nil {
			return err
		}
	}
	if v, ok := raw["uniqueId"]; ok {
		if err := json.Unmarshal(v, &bc.UniqueId); err != nil {
			return err
		}
	}
	if v, ok := raw["status"]; ok {
		if err := json.Unmarshal(v, &bc.Status); err != nil {
			return err
		}
	}

	var parsedBatches []json.RawMessage
	if batchesRaw, ok := raw["batches"]; ok {
		if err := json.Unmarshal(batchesRaw, &parsedBatches); err != nil {
			return err
		}
	}

	// Convert batches from raw JSON to [][]UpgradeTarget
	bc.Batches = make([][]UpgradeTarget, 0, len(parsedBatches))
	for i, batchRaw := range parsedBatches {
		var batchItems []json.RawMessage
		if err := json.Unmarshal(batchRaw, &batchItems); err != nil {
			// Log and skip malformed batch instead of panicking
			klog.Warningf("batch %d is not an array, skipping: %v", i, err)
			continue
		}

		// Use append instead of pre-allocation to avoid index corruption
		targets := make([]UpgradeTarget, 0, len(batchItems))
		for j, itemRaw := range batchItems {
			var target UpgradeTarget
			if err := json.Unmarshal(itemRaw, &target); err != nil {
				// Log the error instead of silently skipping
				klog.Warningf("failed to convert batch item %d: %v", j, err)
				continue
			}
			targets = append(targets, target)
		}
		bc.Batches = append(bc.Batches, targets)
	}

	if bc.Kind == "" {
		bc.Kind = UpgradeKindMountPod
	}

	return nil
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
	Skip    UpgradeStatus = "skip"
)

type UpgradeKind string

const (
	UpgradeKindMountPod UpgradeKind = "mountPod"
	UpgradeKindSidecar  UpgradeKind = "sidecar"
)

type UpgradeMethod string

const (
	UpgradeMethodRecreate UpgradeMethod = "recreate"
	UpgradeMethodBinary   UpgradeMethod = "binary"
)

// UpgradeTarget is a generalized data structure for both Mount Pod and Sidecar upgrades.
// It contains only the minimal persisted fields needed to identify the target.
type UpgradeTarget struct {
	Namespace     string        `json:"namespace"`
	Name          string        `json:"name"`
	ContainerName string        `json:"containerName,omitempty"` // for sidecars
	Node          string        `json:"node,omitempty"`
	CSINodePod    string        `json:"csiNodePod,omitempty"`
	UniqueID      string        `json:"uniqueId,omitempty"`
	Status        UpgradeStatus `json:"status,omitempty"`
}

// Key returns a key based on pod name and container name.
func (t *UpgradeTarget) Key() string {
	if t.Name == "" {
		return ""
	}
	if t.ContainerName == "" {
		return t.Name
	}
	return t.Name + "/" + t.ContainerName
}

// SelectSidecarUpgradeTargets filters sidecar upgrade targets from pods.
// It is extracted for reuse by dashboard API and kubectl plugin.
// used by kubectl plugin
func SelectSidecarUpgradeTargets(
	pods []corev1.Pod,
	pvcMap map[string]corev1.PersistentVolumeClaim,
	secretMap map[types.NamespacedName]corev1.Secret,
) ([]UpgradeTarget, []UpgradeTarget, error) {
	eligible := make([]UpgradeTarget, 0)
	skipped := make([]UpgradeTarget, 0)
	for i := range pods {
		pod := &pods[i]
		if !isSidecarPodReady(pod) || pod.DeletionTimestamp != nil {
			continue
		}

		containers := sidecarContainers(pod)
		for j := range containers {
			container := &containers[j]
			currentImage := EffectiveSidecarImage(pod, *container)
			targetImage, _, err := ResolveSidecarTargetImageFromObjects(pod, container, pvcMap, secretMap)
			if err != nil {
				return nil, nil, err
			}
			if targetImage == "" {
				continue
			}

			target := UpgradeTarget{
				Namespace:     pod.Namespace,
				Name:          pod.Name,
				ContainerName: container.Name,
				Node:          pod.Spec.NodeName,
			}
			if currentImage != targetImage {
				eligible = append(eligible, target)
			} else {
				skipped = append(skipped, target)
			}
		}
	}

	return eligible, skipped, nil
}

func isSidecarPodReady(pod *corev1.Pod) bool {
	conditionsTrue := 0
	for _, cond := range pod.Status.Conditions {
		if cond.Status == corev1.ConditionTrue && (cond.Type == corev1.ContainersReady || cond.Type == corev1.PodReady) {
			conditionsTrue++
		}
	}
	return conditionsTrue == 2
}

func sidecarContainers(pod *corev1.Pod) []corev1.Container {
	if pod.Labels == nil || pod.Labels[common.InjectSidecarDone] != common.True {
		return nil
	}
	containers := make([]corev1.Container, 0)
	for _, container := range pod.Spec.Containers {
		if isSidecarContainerName(container.Name) {
			containers = append(containers, container)
		}
	}
	if pod.Spec.RestartPolicy == corev1.RestartPolicyAlways {
		for _, container := range pod.Spec.InitContainers {
			if isSidecarContainerName(container.Name) {
				containers = append(containers, container)
			}
		}
	}
	return containers
}

func isSidecarContainerName(name string) bool {
	if name == common.MountContainerName {
		return true
	}

	prefix := common.MountContainerName + "-"
	if !strings.HasPrefix(name, prefix) || len(name) == len(prefix) {
		return false
	}
	for _, c := range name[len(prefix):] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type SidecarBinaryUpgradeInfo struct {
	Image      string      `json:"image"`
	UpgradedAt metav1.Time `json:"upgradedAt"`
}

func ParseSidecarBinaryUpgradeAnnotation(pod *corev1.Pod) map[string]SidecarBinaryUpgradeInfo {
	result := make(map[string]SidecarBinaryUpgradeInfo)
	if pod == nil || pod.Annotations == nil {
		return result
	}
	raw := pod.Annotations[common.SidecarBinaryUpgradeAnnotationKey]
	if raw == "" || json.Unmarshal([]byte(raw), &result) != nil {
		return make(map[string]SidecarBinaryUpgradeInfo)
	}
	return result
}

func EffectiveSidecarImage(pod *corev1.Pod, container corev1.Container) string {
	if info, ok := ParseSidecarBinaryUpgradeAnnotation(pod)[container.Name]; ok && info.Image != "" {
		return info.Image
	}
	return container.Image
}

func ResolveSidecarTargetImageFromObjects(
	pod *corev1.Pod,
	container *corev1.Container,
	pvcMap map[string]corev1.PersistentVolumeClaim,
	secretMap map[types.NamespacedName]corev1.Secret,
) (string, bool, error) {
	return ResolveSidecarTargetImage(pod, container,
		func(secretName, namespace string) (*corev1.Secret, error) {
			secret, ok := secretMap[types.NamespacedName{Name: secretName, Namespace: namespace}]
			if !ok {
				return nil, nil
			}
			return &secret, nil
		},
		func(name, namespace string) (*corev1.PersistentVolumeClaim, error) {
			pvc, ok := pvcMap[name]
			if !ok {
				return nil, nil
			}
			return &pvc, nil
		},
	)
}

// ResolveSidecarTargetImage calculates a sidecar target image using resource getters.
func ResolveSidecarTargetImage(
	pod *corev1.Pod,
	container *corev1.Container,
	getSecret func(name, namespace string) (*corev1.Secret, error),
	getPVC func(name, namespace string) (*corev1.PersistentVolumeClaim, error),
) (string, bool, error) {
	if pod == nil || container == nil {
		return "", false, fmt.Errorf("pod and container are required")
	}
	if GlobalConfig == nil {
		return "", false, fmt.Errorf("global config is not loaded")
	}
	if getSecret == nil || getPVC == nil {
		return "", false, fmt.Errorf("sidecar resource getters are required")
	}

	var isCe bool
	var hasSetting bool
	var pvcName string
	for _, vm := range container.VolumeMounts {
		if vm.MountPath != "/jfs-scripts" {
			continue
		}
		for _, vol := range pod.Spec.Volumes {
			if vol.Name != vm.Name || vol.Secret == nil {
				continue
			}
			secret, err := getSecret(vol.Secret.SecretName, pod.Namespace)
			if err != nil {
				return "", false, err
			}
			if secret == nil {
				continue
			}
			if raw, ok := secret.Data["jfsSettings"]; ok && len(raw) > 0 {
				setting := &JfsSetting{}
				if err := setting.Load(string(raw)); err == nil {
					isCe, hasSetting = setting.IsCe, true
				}
			}
			for _, owner := range secret.OwnerReferences {
				if owner.Kind == "PersistentVolumeClaim" {
					pvcName = owner.Name
					break
				}
			}
			break
		}
		if hasSetting || pvcName != "" {
			break
		}
	}
	if pvcName == "" || !hasSetting {
		return "", false, nil
	}
	pvc, err := getPVC(pvcName, pod.Namespace)
	if err != nil {
		return "", false, err
	}
	if pvc == nil {
		return "", false, nil
	}
	return GlobalConfig.GenMountPodPatch(JfsSetting{PVC: pvc, IsCe: isCe}, false, nil).Image, isCe, nil
}

// used by kubectl plugin
func NewBatchConfig(pods []corev1.Pod, parallel int, ignoreError bool, recreate bool, nodeName string, uniqueId string, csiNodes []corev1.Pod) *BatchConfig {
	batchConf := &BatchConfig{
		Parallel:    parallel,
		IgnoreError: ignoreError,
		NoRecreate:  !recreate,
		Kind:        UpgradeKindMountPod,
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
	batches := make([][]UpgradeTarget, (len(pods)+parallel-1)/parallel)
	for _, pod := range pods {
		// Get CSI node name, handle missing CSI node gracefully
		csiNodePodName := ""
		if csiNode, exists := csiNodesMap[pod.Spec.NodeName]; exists {
			csiNodePodName = csiNode.Name
		} else {
			klog.Warningf("CSI node pod not found for node %s (pod %s/%s)", pod.Spec.NodeName, pod.Namespace, pod.Name)
		}

		target := UpgradeTarget{
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			Node:       pod.Spec.NodeName,
			CSINodePod: csiNodePodName,
		}
		batches[j] = append(batches[j], target)
		index += 1

		if index == parallel {
			j += 1
			index = 0
		}
	}
	batchConf.Batches = batches
	return batchConf
}

// NewBatchConfigForSidecars creates a BatchConfig from sidecar upgrade targets
// used by kubectl plugin
func NewBatchConfigForSidecars(targets []UpgradeTarget, parallel int, ignoreError bool, namespace string) *BatchConfig {
	batchConf := &BatchConfig{
		Parallel:    parallel,
		IgnoreError: ignoreError,
		NoRecreate:  false, // binary upgrade
		Kind:        UpgradeKindSidecar,
		Namespace:   namespace,
		Node:        "",
		UniqueId:    "",
	}

	if len(targets) == 0 {
		batchConf.Batches = [][]UpgradeTarget{}
		return batchConf
	}

	// Sort targets by node and pod name for consistency
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Node != targets[j].Node {
			return targets[i].Node < targets[j].Node
		}
		if targets[i].Namespace != targets[j].Namespace {
			return targets[i].Namespace < targets[j].Namespace
		}
		if targets[i].Name != targets[j].Name {
			return targets[i].Name < targets[j].Name
		}
		return targets[i].ContainerName < targets[j].ContainerName
	})

	index := 0
	j := 0
	batches := make([][]UpgradeTarget, (len(targets)+parallel-1)/parallel)
	for _, target := range targets {
		batches[j] = append(batches[j], target)
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
	// Handle nil annotations safely
	iUniqueID := ""
	if p[i].Annotations != nil {
		iUniqueID = p[i].Annotations[common.UniqueId]
	}
	jUniqueID := ""
	if p[j].Annotations != nil {
		jUniqueID = p[j].Annotations[common.UniqueId]
	}
	return iUniqueID < jUniqueID
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
	data := []byte(cm.Data["upgrade"])
	if compressed, ok := cm.BinaryData["upgrade"]; ok {
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		data, err = io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
	}

	err := json.Unmarshal(data, cfg)
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

func setUpgradeConfigData(cfg *corev1.ConfigMap, config *BatchConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if len(data) < corev1.MaxSecretSize/2 {
		cfg.Data = map[string]string{"upgrade": string(data)}
		cfg.BinaryData = nil
		return nil
	}

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if compressed.Len() > corev1.MaxSecretSize {
		return fmt.Errorf("compressed upgrade config is too large: %d bytes", compressed.Len())
	}
	cfg.Data = nil
	cfg.BinaryData = map[string][]byte{"upgrade": compressed.Bytes()}
	return nil
}

// used by kubectl plugin
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
	if cfg == nil {
		cfg = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configName,
				Namespace: Namespace,
				Labels: map[string]string{
					common.PodTypeKey: common.ConfigTypeValue,
				},
			},
		}
		if err := setUpgradeConfigData(cfg, config); err != nil {
			return nil, err
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
	if err := setUpgradeConfigData(cfg, config); err != nil {
		return nil, err
	}
	return cfg, client.UpdateConfigMap(ctx, cfg)
}

// used by kubectl plugin
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

// IsPodUpgradeOngoing checks if a pod's upgrade status indicates it's still in progress
func IsPodUpgradeOngoing(status UpgradeStatus) bool {
	return status != Success && status != Fail && status != Stop && status != Skip
}

// filterPodsFromConfigs extracts pod names that are in ongoing upgrades from the given configs
func filterPodsFromConfigs(configs map[string]*BatchConfig) map[string]struct{} {
	podsInOngoingJobs := make(map[string]struct{})
	for _, cfg := range configs {
		// Only consider pods from configs that are still in progress (not finished)
		if !IsPodUpgradeOngoing(cfg.Status) {
			continue
		}
		for _, batch := range cfg.Batches {
			for _, target := range batch {
				if target.Name != "" {
					podsInOngoingJobs[target.Name] = struct{}{}
				}
			}
		}
	}
	return podsInOngoingJobs
}

// filterTargetKeysFromConfigs extracts target keys that are in ongoing upgrades from the given configs
func filterTargetKeysFromConfigs(configs map[string]*BatchConfig) map[string]struct{} {
	keysInOngoingJobs := make(map[string]struct{})
	for _, cfg := range configs {
		// Only consider targets from configs that are still in progress (not finished)
		if !IsPodUpgradeOngoing(cfg.Status) {
			continue
		}
		for _, batch := range cfg.Batches {
			for _, target := range batch {
				keysInOngoingJobs[target.Key()] = struct{}{}
			}
		}
	}
	return keysInOngoingJobs
}

// FilterTargetsNotInOngoingUpgrade filters out targets that are currently in ongoing upgrade tasks.
// It uses UpgradeTarget.Key() for conflict detection, which handles both pod-level and container-level conflicts.
// Returns the filtered targets list and a list of targets that were skipped.
// used by kubectl plugin
func FilterTargetsNotInOngoingUpgrade(ctx context.Context, client *k8s.K8sClient, targets []UpgradeTarget) ([]UpgradeTarget, []UpgradeTarget, error) {
	if len(targets) == 0 {
		return targets, nil, nil
	}

	// List all upgrade jobs
	s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.JfsJobKind: common.KindOfUpgrade,
		},
	})
	jobList, err := client.BatchV1().Jobs(Namespace).List(ctx, metav1.ListOptions{LabelSelector: s.String()})
	if err != nil {
		return nil, nil, err
	}

	if len(jobList.Items) == 0 {
		return targets, nil, nil
	}

	// Find running jobs and their corresponding configmap names
	runningConfigNames := make(map[string]struct{})
	for _, job := range jobList.Items {
		// Check if job is still running, not completed or failed
		if job.Status.CompletionTime == nil && job.Status.Failed == 0 {
			// Job is still running, extract configmap name from label
			if configName, ok := job.Labels[common.JfsUpgradeConfig]; ok && configName != "" {
				runningConfigNames[configName] = struct{}{}
			}
		}
	}

	if len(runningConfigNames) == 0 {
		return targets, nil, nil
	}

	// Get all upgrade configurations
	configs, err := GetAllUpgradeConfigs(ctx, client)
	if err != nil {
		return nil, nil, err
	}

	// Filter to only keep configs that are in running jobs
	activeConfigs := make(map[string]*BatchConfig)
	for configName, cfg := range configs {
		if _, isRunning := runningConfigNames[configName]; isRunning {
			activeConfigs[configName] = cfg
		}
	}

	// Collect all target keys that are in ongoing upgrade jobs
	keysInOngoingJobs := filterTargetKeysFromConfigs(activeConfigs)

	if len(keysInOngoingJobs) == 0 {
		return targets, nil, nil
	}

	// Filter targets and collect skipped targets
	skippedTargets := make([]UpgradeTarget, 0)
	filteredTargets := make([]UpgradeTarget, 0, len(targets))
	for _, target := range targets {
		if _, exists := keysInOngoingJobs[target.Key()]; exists {
			skippedTargets = append(skippedTargets, target)
			continue
		}
		filteredTargets = append(filteredTargets, target)
	}

	return filteredTargets, skippedTargets, nil
}

// FilterPodsNotInOngoingUpgrade filters out pods that are currently in ongoing upgrade tasks.
// It lists all upgrade jobs, checks which ones are still running, and extracts the configmap
// names from the running jobs' labels. Only pods from those configs are filtered out.
// Returns the filtered pod list and a list of pod names that were skipped.
// used by kubectl plugin
func FilterPodsNotInOngoingUpgrade(ctx context.Context, client *k8s.K8sClient, pods []corev1.Pod) ([]corev1.Pod, []string, error) {
	if len(pods) == 0 {
		return pods, nil, nil
	}

	// List all upgrade jobs
	s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.JfsJobKind: common.KindOfUpgrade,
		},
	})
	jobList, err := client.BatchV1().Jobs(Namespace).List(ctx, metav1.ListOptions{LabelSelector: s.String()})
	if err != nil {
		return nil, nil, err
	}

	if len(jobList.Items) == 0 {
		return pods, nil, nil
	}

	// Find running jobs and their corresponding configmap names
	runningConfigNames := make(map[string]struct{})
	for _, job := range jobList.Items {
		// Check if job is still running, not completed or failed
		if job.Status.CompletionTime == nil && job.Status.Failed == 0 {
			// Job is still running, extract configmap name from label
			if configName, ok := job.Labels[common.JfsUpgradeConfig]; ok && configName != "" {
				runningConfigNames[configName] = struct{}{}
			}
		}
	}

	if len(runningConfigNames) == 0 {
		return pods, nil, nil
	}

	// Get all upgrade configurations
	configs, err := GetAllUpgradeConfigs(ctx, client)
	if err != nil {
		return nil, nil, err
	}

	// Filter to only keep configs that are in running jobs
	activeConfigs := make(map[string]*BatchConfig)
	for configName, cfg := range configs {
		if _, isRunning := runningConfigNames[configName]; isRunning {
			activeConfigs[configName] = cfg
		}
	}

	// Collect all pod names that are in ongoing upgrade jobs
	podsInOngoingJobs := filterPodsFromConfigs(activeConfigs)

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

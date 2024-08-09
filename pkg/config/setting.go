/*
Copyright 2021 Juicedata Inc

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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/security"
)

const (
	podInfoName      = "csi.storage.k8s.io/pod.name"
	podInfoNamespace = "csi.storage.k8s.io/pod.namespace"
)

type JfsSetting struct {
	HashVal string `json:"-"`
	IsCe    bool
	UsePod  bool

	UUID               string
	Name               string               `json:"name"`
	MetaUrl            string               `json:"metaurl"`
	Source             string               `json:"source"`
	Storage            string               `json:"storage"`
	FormatOptions      string               `json:"format-options"`
	CachePVCs          []CachePVC           // PVC using by mount pod
	CacheEmptyDir      *CacheEmptyDir       // EmptyDir using by mount pod
	CacheInlineVolumes []*CacheInlineVolume // InlineVolume using by mount pod
	CacheDirs          []string             // hostPath using by mount pod
	ClientConfPath     string               `json:"-"`

	// put in secret
	SecretKey     string            `json:"secret-key,omitempty"`
	SecretKey2    string            `json:"secret-key2,omitempty"`
	Token         string            `json:"token,omitempty"`
	Passphrase    string            `json:"passphrase,omitempty"`
	Envs          map[string]string `json:"envs_map,omitempty"`
	EncryptRsaKey string            `json:"encrypt_rsa_key,omitempty"`
	InitConfig    string            `json:"initconfig,omitempty"`
	Configs       map[string]string `json:"configs_map,omitempty"`

	// put in volCtx
	DeletedDelay string   `json:"deleted_delay"`
	CleanCache   bool     `json:"clean_cache"`
	HostPath     []string `json:"host_path"`

	// mount
	VolumeId   string   // volumeHandle of PV
	UniqueId   string   // mount pod name is generated by uniqueId
	MountPath  string   // mountPath of mount pod or process mount
	TargetPath string   // which bind to container path
	Options    []string // mount options
	FormatCmd  string   // format or auth
	SubPath    string   // subPath which is to be created or deleted
	SecretName string   // secret with JuiceFS volume credentials

	Attr *PodAttr

	PV  *corev1.PersistentVolume      `json:"-"`
	PVC *corev1.PersistentVolumeClaim `json:"-"`
}

type PodAttr struct {
	Namespace            string
	MountPointPath       string
	JFSConfigPath        string
	JFSMountPriorityName string
	ServiceAccountName   string

	Resources corev1.ResourceRequirements

	Labels                        map[string]string     `json:"labels,omitempty"`
	Annotations                   map[string]string     `json:"annotations,omitempty"`
	LivenessProbe                 *corev1.Probe         `json:"livenessProbe,omitempty"`
	ReadinessProbe                *corev1.Probe         `json:"readinessProbe,omitempty"`
	StartupProbe                  *corev1.Probe         `json:"startupProbe,omitempty"`
	Lifecycle                     *corev1.Lifecycle     `json:"lifecycle,omitempty"`
	TerminationGracePeriodSeconds *int64                `json:"terminationGracePeriodSeconds,omitempty"`
	Volumes                       []corev1.Volume       `json:"volumes,omitempty"`
	VolumeDevices                 []corev1.VolumeDevice `json:"volumeDevices,omitempty"`
	VolumeMounts                  []corev1.VolumeMount  `json:"volumeMounts,omitempty"`
	Env                           []corev1.EnvVar       `json:"env,omitempty"`

	// inherit from csi
	Image            string
	HostNetwork      bool
	HostAliases      []corev1.HostAlias
	HostPID          bool
	HostIPC          bool
	DNSConfig        *corev1.PodDNSConfig
	DNSPolicy        corev1.DNSPolicy
	ImagePullSecrets []corev1.LocalObjectReference
	PreemptionPolicy *corev1.PreemptionPolicy
	Tolerations      []corev1.Toleration
}

// info of app pod
type AppInfo struct {
	Name      string
	Namespace string
}

type CachePVC struct {
	PVCName string
	Path    string
}

type CacheEmptyDir struct {
	Medium    string
	SizeLimit resource.Quantity
	Path      string
}

type CacheInlineVolume struct {
	CSI  *corev1.CSIVolumeSource
	Path string
}

func ParseSetting(secrets, volCtx map[string]string, options []string, usePod bool, pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim) (*JfsSetting, error) {
	jfsSetting := JfsSetting{
		Options: []string{},
	}
	if options != nil {
		jfsSetting.Options = options
	}
	if secrets == nil {
		return &jfsSetting, nil
	}

	secretStr, err := json.Marshal(secrets)
	if err != nil {
		return nil, err
	}
	if err := parseYamlOrJson(string(secretStr), &jfsSetting); err != nil {
		return nil, err
	}

	if secrets["name"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty name")
	}
	jfsSetting.Name = secrets["name"]
	jfsSetting.Storage = secrets["storage"]
	jfsSetting.Envs = make(map[string]string)
	jfsSetting.Configs = make(map[string]string)
	jfsSetting.ClientConfPath = DefaultClientConfPath
	jfsSetting.CacheDirs = []string{}
	jfsSetting.CachePVCs = []CachePVC{}
	jfsSetting.PV = pv
	jfsSetting.PVC = pvc

	jfsSetting.UsePod = usePod
	jfsSetting.Source = jfsSetting.Name
	if source, ok := secrets["metaurl"]; ok {
		jfsSetting.MetaUrl = source
		jfsSetting.IsCe = ok
		// Default use redis:// scheme
		if !strings.Contains(source, "://") {
			source = "redis://" + source
		}
		jfsSetting.Source = source
	}

	if secrets["secretkey"] != "" {
		jfsSetting.SecretKey = secrets["secretkey"]
	}
	if secrets["secretkey2"] != "" {
		jfsSetting.SecretKey2 = secrets["secretkey2"]
	}

	if secrets["configs"] != "" {
		configStr := secrets["configs"]
		configs := make(map[string]string)
		klog.V(6).Infof("Get configs in secret: %v", configStr)
		if err := parseYamlOrJson(configStr, &configs); err != nil {
			return nil, err
		}
		jfsSetting.Configs = configs
	}

	if secrets["envs"] != "" {
		envStr := secrets["envs"]
		env := make(map[string]string)
		klog.V(6).Infof("Get envs in secret: %v", envStr)
		if err := parseYamlOrJson(envStr, &env); err != nil {
			return nil, err
		}
		jfsSetting.Envs = env
	}

	if volCtx != nil {
		// subPath
		if volCtx["subPath"] != "" {
			jfsSetting.SubPath = volCtx["subPath"]
		}

		if volCtx[cleanCache] == "true" {
			jfsSetting.CleanCache = true
		}
		delay := volCtx[deleteDelay]
		if delay != "" {
			if _, err := time.ParseDuration(delay); err != nil {
				return nil, fmt.Errorf("can't parse delay time %s", delay)
			}
			jfsSetting.DeletedDelay = delay
		}

		var hostPaths []string
		if volCtx[mountPodHostPath] != "" {
			for _, v := range strings.Split(volCtx[mountPodHostPath], ",") {
				p := strings.TrimSpace(v)
				if p != "" {
					hostPaths = append(hostPaths, strings.TrimSpace(v))
				}
			}
			jfsSetting.HostPath = hostPaths
		}
	}

	if err := GenPodAttrWithCfg(&jfsSetting, volCtx); err != nil {
		return nil, fmt.Errorf("GenPodAttrWithCfg error: %v", err)
	}
	if err := genAndValidOptions(&jfsSetting, options); err != nil {
		return nil, fmt.Errorf("genAndValidOptions error: %v", err)
	}
	if err := genCacheDirs(&jfsSetting, volCtx); err != nil {
		return nil, fmt.Errorf("genCacheDirs error: %v", err)
	}
	return &jfsSetting, nil
}

func genCacheDirs(jfsSetting *JfsSetting, volCtx map[string]string) error {
	cacheDirsInContainer := []string{}
	var err error
	// parse pvc of cache
	if volCtx != nil && volCtx[cachePVC] != "" {
		cachePVCs := strings.Split(strings.TrimSpace(volCtx[cachePVC]), ",")
		for i, pvc := range cachePVCs {
			if pvc == "" {
				continue
			}
			volPath := fmt.Sprintf("/var/jfsCache-%d", i)
			jfsSetting.CachePVCs = append(jfsSetting.CachePVCs, CachePVC{
				PVCName: pvc,
				Path:    volPath,
			})
			cacheDirsInContainer = append(cacheDirsInContainer, volPath)
		}
	}
	// parse emptydir of cache
	if volCtx != nil {
		if _, ok := volCtx[cacheEmptyDir]; ok {
			volPath := "/var/jfsCache-emptyDir"
			cacheDirsInContainer = append(cacheDirsInContainer, volPath)
			cacheEmptyDirs := strings.Split(strings.TrimSpace(volCtx[cacheEmptyDir]), ":")
			var (
				medium    string
				sizeLimit string
			)
			if len(cacheEmptyDirs) == 1 {
				medium = strings.TrimSpace(cacheEmptyDirs[0])
			}
			if len(cacheEmptyDirs) == 2 {
				medium = strings.TrimSpace(cacheEmptyDirs[0])
				sizeLimit = strings.TrimSpace(cacheEmptyDirs[1])
			}
			jfsSetting.CacheEmptyDir = &CacheEmptyDir{
				Medium: medium,
				Path:   volPath,
			}
			klog.Infof("sizeLimit of emptyDir is %s", sizeLimit)
			if sizeLimit != "" {
				if jfsSetting.CacheEmptyDir.SizeLimit, err = resource.ParseQuantity(sizeLimit); err != nil {
					return err
				}
			}
		}
	}
	// parse inline volume of cache
	if volCtx != nil {
		if _, ok := volCtx[cacheInlineVolume]; ok {
			inlineVolumes := []*corev1.CSIVolumeSource{}
			err = json.Unmarshal([]byte(volCtx[cacheInlineVolume]), &inlineVolumes)
			if err != nil {
				return fmt.Errorf("parse cache inline volume error: %v", err)
			}
			jfsSetting.CacheInlineVolumes = make([]*CacheInlineVolume, 0)
			klog.V(6).Infof("get cache inline volume: %v", inlineVolumes)

			for i, inlineVolume := range inlineVolumes {
				volPath := fmt.Sprintf("/var/jfsCache-inlineVolume-%d", i)
				cacheDirsInContainer = append(cacheDirsInContainer, volPath)
				jfsSetting.CacheInlineVolumes = append(jfsSetting.CacheInlineVolumes, &CacheInlineVolume{
					CSI:  inlineVolume,
					Path: volPath,
				})
			}
		}
	}
	// parse cache dirs in option
	var cacheDirsInOptions []string
	options := jfsSetting.Options
	for i, o := range options {
		if strings.HasPrefix(o, "cache-dir") {
			optValPair := strings.Split(o, "=")
			if len(optValPair) != 2 {
				continue
			}
			cacheDirsInOptions = strings.Split(strings.TrimSpace(optValPair[1]), ":")
			cacheDirsInContainer = append(cacheDirsInContainer, cacheDirsInOptions...)
			options = append(options[:i], options[i+1:]...)
			break
		}
	}
	if len(cacheDirsInContainer) == 0 {
		// set default cache dir
		cacheDirsInOptions = []string{"/var/jfsCache"}
	}
	for _, d := range cacheDirsInOptions {
		if d != "memory" {
			// filter out "memory"
			jfsSetting.CacheDirs = append(jfsSetting.CacheDirs, d)
		}
	}

	// replace cacheDir in option
	if len(cacheDirsInContainer) > 0 {
		options = append(options, fmt.Sprintf("cache-dir=%s", strings.Join(cacheDirsInContainer, ":")))
		jfsSetting.Options = options
	}
	return nil
}

func genAndValidOptions(JfsSetting *JfsSetting, options []string) error {
	mountOptions := []string{}
	for _, option := range options {
		mountOption := strings.TrimSpace(option)
		ops := strings.Split(mountOption, "=")
		if len(ops) > 2 {
			return fmt.Errorf("invalid mount option: %s", mountOption)
		}
		if len(ops) == 2 {
			mountOption = fmt.Sprintf("%s=%s", strings.TrimSpace(ops[0]), strings.TrimSpace(ops[1]))
		}
		if mountOption == "writeback" {
			klog.Warningf("writeback is not suitable in CSI, please do not use it. volumeId: %s", JfsSetting.VolumeId)
		}
		if len(ops) == 2 && ops[0] == "buffer-size" {
			memLimit := JfsSetting.Attr.Resources.Limits[corev1.ResourceMemory]
			memLimitByte := memLimit.Value()

			// buffer-size is in MiB, turn to byte
			bufferSize, err := util.ParseToBytes(ops[1])
			if err != nil {
				return fmt.Errorf("invalid mount option: %s", mountOption)
			}
			if bufferSize > uint64(memLimitByte) {
				return fmt.Errorf("buffer-size %s MiB is greater than pod memory limit %s", ops[1], memLimit.String())
			}
		}
		mountOptions = append(mountOptions, mountOption)
	}
	JfsSetting.Options = mountOptions
	return nil
}

func GenPodAttrWithCfg(setting *JfsSetting, volCtx map[string]string) error {
	var err error
	var attr *PodAttr
	if setting.Attr != nil {
		attr = setting.Attr
	} else {
		attr = &PodAttr{
			Namespace:            Namespace,
			MountPointPath:       MountPointPath,
			JFSConfigPath:        JFSConfigPath,
			JFSMountPriorityName: JFSMountPriorityName,
			HostNetwork:          CSIPod.Spec.HostNetwork,
			HostAliases:          CSIPod.Spec.HostAliases,
			HostPID:              CSIPod.Spec.HostPID,
			HostIPC:              CSIPod.Spec.HostIPC,
			DNSConfig:            CSIPod.Spec.DNSConfig,
			DNSPolicy:            CSIPod.Spec.DNSPolicy,
			ImagePullSecrets:     CSIPod.Spec.ImagePullSecrets,
			Tolerations:          CSIPod.Spec.Tolerations,
			PreemptionPolicy:     CSIPod.Spec.PreemptionPolicy,
			ServiceAccountName:   CSIPod.Spec.ServiceAccountName,
			Resources:            getDefaultResource(),
			Labels:               make(map[string]string),
			Annotations:          make(map[string]string),
		}
		if setting.IsCe {
			attr.Image = DefaultCEMountImage
		} else {
			attr.Image = DefaultEEMountImage
		}
		setting.Attr = attr
	}
	if JFSMountPreemptionPolicy != "" {
		policy := corev1.PreemptionPolicy(JFSMountPreemptionPolicy)
		attr.PreemptionPolicy = &policy
	}

	if volCtx != nil {
		if v, ok := volCtx[mountPodImageKey]; ok && v != "" {
			attr.Image = v
		}
		if v, ok := volCtx[mountPodServiceAccount]; ok && v != "" {
			attr.ServiceAccountName = v
		}
		cpuLimit := volCtx[MountPodCpuLimitKey]
		memoryLimit := volCtx[MountPodMemLimitKey]
		cpuRequest := volCtx[MountPodCpuRequestKey]
		memoryRequest := volCtx[MountPodMemRequestKey]
		attr.Resources, err = ParsePodResources(cpuLimit, memoryLimit, cpuRequest, memoryRequest)
		if err != nil {
			klog.Errorf("Parse resource error: %v", err)
			return err
		}
		if v, ok := volCtx[mountPodLabelKey]; ok && v != "" {
			ctxLabel := make(map[string]string)
			if err := parseYamlOrJson(v, &ctxLabel); err != nil {
				return err
			}
			for k, v := range ctxLabel {
				attr.Labels[k] = v
			}
		}
		if v, ok := volCtx[mountPodAnnotationKey]; ok && v != "" {
			ctxAnno := make(map[string]string)
			if err := parseYamlOrJson(v, &ctxAnno); err != nil {
				return err
			}
			for k, v := range ctxAnno {
				attr.Annotations[k] = v
			}
		}
	}

	// apply config patch
	applyAttrPatch(attr, setting)
	return nil
}

// GenPodAttrWithMountPod generate pod attr with mount pod
// Return the latest pod attributes following the priorities below:
//
// 1. original mount pod
// 2. pvc annotations
// 3. global config
func GenPodAttrWithMountPod(ctx context.Context, client *k8sclient.K8sClient, mountPod *corev1.Pod) (*PodAttr, error) {
	attr := &PodAttr{
		Namespace:            mountPod.Namespace,
		MountPointPath:       MountPointPath,
		JFSConfigPath:        JFSConfigPath,
		JFSMountPriorityName: JFSMountPriorityName,
		HostNetwork:          mountPod.Spec.HostNetwork,
		HostAliases:          mountPod.Spec.HostAliases,
		HostPID:              mountPod.Spec.HostPID,
		HostIPC:              mountPod.Spec.HostIPC,
		DNSConfig:            mountPod.Spec.DNSConfig,
		DNSPolicy:            mountPod.Spec.DNSPolicy,
		ImagePullSecrets:     mountPod.Spec.ImagePullSecrets,
		Tolerations:          mountPod.Spec.Tolerations,
		PreemptionPolicy:     mountPod.Spec.PreemptionPolicy,
		ServiceAccountName:   mountPod.Spec.ServiceAccountName,
		Labels:               make(map[string]string),
		Annotations:          make(map[string]string),
	}
	if mountPod.Spec.Containers != nil && len(mountPod.Spec.Containers) > 0 {
		attr.Image = mountPod.Spec.Containers[0].Image
		attr.Resources = mountPod.Spec.Containers[0].Resources
	}
	for k, v := range mountPod.Labels {
		attr.Labels[k] = v
	}
	for k, v := range mountPod.Annotations {
		attr.Annotations[k] = v
	}
	pvName := mountPod.Annotations[UniqueId]
	pv, err := client.GetPersistentVolume(ctx, pvName)
	if err != nil {
		klog.Errorf("Get pv %s error: %v", pvName, err)
		return nil, err
	}
	pvc, err := client.GetPersistentVolumeClaim(ctx, pv.Spec.ClaimRef.Name, pv.Spec.ClaimRef.Namespace)
	if err != nil {
		klog.Errorf("Get pvc %s/%s error: %v", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name, err)
		return nil, err
	}
	cpuLimit := pvc.Annotations[MountPodCpuLimitKey]
	memoryLimit := pvc.Annotations[MountPodMemLimitKey]
	cpuRequest := pvc.Annotations[MountPodCpuRequestKey]
	memoryRequest := pvc.Annotations[MountPodMemRequestKey]
	resources, err := ParsePodResources(cpuLimit, memoryLimit, cpuRequest, memoryRequest)
	if err != nil {
		return nil, fmt.Errorf("parse pvc resources error: %v", err)
	}
	attr.Resources = resources
	setting := &JfsSetting{
		IsCe:      IsCEMountPod(mountPod),
		PV:        pv,
		PVC:       pvc,
		Name:      mountPod.Annotations[JuiceFSUUID],
		VolumeId:  mountPod.Annotations[UniqueId],
		MountPath: filepath.Join(PodMountBase, pvName) + mountPod.Name[len(mountPod.Name)-7:],
	}
	if v, ok := pv.Spec.CSI.VolumeAttributes["subPath"]; ok && v != "" {
		setting.SubPath = v
	}
	// apply config patch
	applyAttrPatch(attr, setting)
	return attr, nil
}

func ParseAppInfo(volCtx map[string]string) (*AppInfo, error) {
	// check kubelet access. If not, should turn `podInfoOnMount` on in csiDriver, and fallback to apiServer
	if !ByProcess && !Webhook && KubeletPort != "" && HostIp != "" {
		port, err := strconv.Atoi(KubeletPort)
		if err != nil {
			return nil, err
		}
		kc, err := k8sclient.NewKubeletClient(HostIp, port)
		if err != nil {
			return nil, err
		}
		if _, err := kc.GetNodeRunningPods(); err != nil {
			if volCtx == nil || volCtx[podInfoName] == "" {
				return nil, fmt.Errorf("can not connect to kubelet, please turn `podInfoOnMount` on in csiDriver, and fallback to apiServer")
			}
		}
	}
	if volCtx != nil {
		return &AppInfo{
			Name:      volCtx[podInfoName],
			Namespace: volCtx[podInfoNamespace],
		}, nil
	}
	return nil, nil
}

func (s *JfsSetting) ParseFormatOptions() ([][]string, error) {
	options := strings.Split(s.FormatOptions, ",")
	parsedFormatOptions := make([][]string, 0, len(options))
	for _, option := range options {
		pair := strings.SplitN(strings.TrimSpace(option), "=", 2)
		if len(pair) == 2 && pair[1] == "" {
			return nil, fmt.Errorf("invalid format options: %s", s.FormatOptions)
		}
		key := strings.TrimSpace(pair[0])
		if key == "" {
			// ignore empty key
			continue
		}
		var value string
		if len(pair) == 1 {
			// single key
			value = ""
		} else {
			value = strings.TrimSpace(pair[1])
		}
		parsedFormatOptions = append(parsedFormatOptions, []string{key, value})
	}
	return parsedFormatOptions, nil
}

func (s *JfsSetting) RepresentFormatOptions(parsedOptions [][]string) []string {
	options := make([]string, 0)
	for _, pair := range parsedOptions {
		option := security.EscapeBashStr(pair[0])
		if pair[1] != "" {
			option = fmt.Sprintf("%s=%s", option, security.EscapeBashStr(pair[1]))
		}
		options = append(options, "--"+option)
	}
	return options
}

func (s *JfsSetting) StripFormatOptions(parsedOptions [][]string, strippedKeys []string) []string {
	options := make([]string, 0)
	strippedMap := make(map[string]bool)
	for _, key := range strippedKeys {
		strippedMap[key] = true
	}

	for _, pair := range parsedOptions {
		option := security.EscapeBashStr(pair[0])
		if pair[1] != "" {
			if strippedMap[pair[0]] {
				option = fmt.Sprintf("%s=${%s}", option, pair[0])
			} else {
				option = fmt.Sprintf("%s=%s", option, security.EscapeBashStr(pair[1]))
			}
		}
		options = append(options, "--"+option)
	}
	return options
}

func parseYamlOrJson(source string, dst interface{}) error {
	if err := yaml.Unmarshal([]byte(source), &dst); err != nil {
		if err := json.Unmarshal([]byte(source), &dst); err != nil {
			return status.Errorf(codes.InvalidArgument,
				"Parse yaml or json error: %v", err)
		}
	}
	return nil
}

func ParseYamlOrJson(source string, dst interface{}) error {
	return parseYamlOrJson(source, dst)
}

func ParsePodResources(cpuLimit, memoryLimit, cpuRequest, memoryRequest string) (corev1.ResourceRequirements, error) {
	podLimit := map[corev1.ResourceName]resource.Quantity{}
	podRequest := map[corev1.ResourceName]resource.Quantity{}
	// set default value
	podLimit[corev1.ResourceCPU] = resource.MustParse(DefaultMountPodCpuLimit)
	podLimit[corev1.ResourceMemory] = resource.MustParse(DefaultMountPodMemLimit)
	podRequest[corev1.ResourceCPU] = resource.MustParse(DefaultMountPodCpuRequest)
	podRequest[corev1.ResourceMemory] = resource.MustParse(DefaultMountPodMemRequest)
	var err error
	if cpuLimit != "" {
		if podLimit[corev1.ResourceCPU], err = resource.ParseQuantity(cpuLimit); err != nil {
			return corev1.ResourceRequirements{}, err
		}
		q := podLimit[corev1.ResourceCPU]
		if res := q.Cmp(*resource.NewQuantity(0, resource.DecimalSI)); res <= 0 {
			delete(podLimit, corev1.ResourceCPU)
		}
	}
	if memoryLimit != "" {
		if podLimit[corev1.ResourceMemory], err = resource.ParseQuantity(memoryLimit); err != nil {
			return corev1.ResourceRequirements{}, err
		}
		q := podLimit[corev1.ResourceMemory]
		if res := q.Cmp(*resource.NewQuantity(0, resource.DecimalSI)); res <= 0 {
			delete(podLimit, corev1.ResourceMemory)
		}
	}
	if cpuRequest != "" {
		if podRequest[corev1.ResourceCPU], err = resource.ParseQuantity(cpuRequest); err != nil {
			return corev1.ResourceRequirements{}, err
		}
		q := podRequest[corev1.ResourceCPU]
		if res := q.Cmp(*resource.NewQuantity(0, resource.DecimalSI)); res <= 0 {
			delete(podRequest, corev1.ResourceCPU)
		}
	}
	if memoryRequest != "" {
		if podRequest[corev1.ResourceMemory], err = resource.ParseQuantity(memoryRequest); err != nil {
			return corev1.ResourceRequirements{}, err
		}
		q := podRequest[corev1.ResourceMemory]
		if res := q.Cmp(*resource.NewQuantity(0, resource.DecimalSI)); res <= 0 {
			delete(podRequest, corev1.ResourceMemory)
		}
	}
	return corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}, nil
}

func getDefaultResource() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(DefaultMountPodCpuLimit),
			corev1.ResourceMemory: resource.MustParse(DefaultMountPodMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(DefaultMountPodCpuRequest),
			corev1.ResourceMemory: resource.MustParse(DefaultMountPodMemRequest),
		},
	}
}

func applyAttrPatch(attr *PodAttr, setting *JfsSetting) {
	// overwrite by mountpod patch
	patch := GlobalConfig.GenMountPodPatch(*setting)
	if patch.Image != "" {
		attr.Image = patch.Image
	}
	if patch.HostNetwork != nil {
		attr.HostNetwork = *patch.HostNetwork
	}
	if patch.HostPID != nil {
		attr.HostPID = *patch.HostPID
	}
	for k, v := range patch.Labels {
		attr.Labels[k] = v
	}
	for k, v := range patch.Annotations {
		attr.Annotations[k] = v
	}
	if patch.Resources != nil {
		attr.Resources = *patch.Resources
	}
	attr.Lifecycle = patch.Lifecycle
	attr.LivenessProbe = patch.LivenessProbe
	attr.ReadinessProbe = patch.ReadinessProbe
	attr.StartupProbe = patch.StartupProbe
	attr.TerminationGracePeriodSeconds = patch.TerminationGracePeriodSeconds
	attr.VolumeDevices = patch.VolumeDevices
	attr.VolumeMounts = patch.VolumeMounts
	attr.Volumes = patch.Volumes
	attr.Env = patch.Env
}

// IsCEMountPod check if the pod is a mount pod of CE
// check mountpod command's has metaurl
func IsCEMountPod(pod *corev1.Pod) bool {
	for _, cmd := range pod.Spec.Containers[0].Command {
		if strings.Contains(cmd, "metaurl") {
			return true
		}
	}
	return false
}

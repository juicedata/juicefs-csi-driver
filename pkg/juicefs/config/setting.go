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
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type JfsSetting struct {
	IsCe   bool
	UsePod bool

	Name    string `json:"name"`
	MetaUrl string `json:"metaurl"`
	Source  string `json:"source"`
	Storage string `json:"storage"`

	Configs map[string]string `json:"configs"`
	Envs    map[string]string `json:"envs"`

	MountPodCpuLimit    string            `json:"mount_pod_cpu_limit"`
	MountPodMemLimit    string            `json:"mount_pod_mem_limit"`
	MountPodCpuRequest  string            `json:"mount_pod_cpu_request"`
	MountPodMemRequest  string            `json:"mount_pod_mem_request"`
	MountPodLabels      map[string]string `json:"mount_pod_labels"`
	MountPodAnnotations map[string]string `json:"mount_pod_annotations"`
}

func ParseSetting(secrets, volCtx map[string]string, usePod bool) (*JfsSetting, error) {
	jfsSetting := JfsSetting{}
	if secrets == nil {
		return &jfsSetting, nil
	}
	if secrets["name"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty name")
	}
	jfsSetting.Name = secrets["name"]
	jfsSetting.Storage = secrets["storage"]

	m, ok := secrets["metaurl"]
	jfsSetting.MetaUrl = m
	jfsSetting.IsCe = ok
	jfsSetting.UsePod = usePod

	if secrets["configs"] != "" {
		configStr := secrets["configs"]
		configs := make(map[string]string)
		if err := parseYamlOrJson(configStr, &configs); err != nil {
			return nil, err
		}
		jfsSetting.Configs = configs
	}

	if secrets["envs"] != "" {
		envStr := secrets["envs"]
		env := make(map[string]string)
		if err := parseYamlOrJson(envStr, &env); err != nil {
			return nil, err
		}
		jfsSetting.Envs = env
	}

	labels := make(map[string]string)
	if MountLabels != "" {
		if err := parseYamlOrJson(MountLabels, &labels); err != nil {
			return nil, err
		}
	}

	if volCtx != nil {
		jfsSetting.MountPodCpuLimit = volCtx[mountPodCpuLimitKey]
		jfsSetting.MountPodMemLimit = volCtx[mountPodMemLimitKey]
		jfsSetting.MountPodCpuRequest = volCtx[mountPodCpuRequestKey]
		jfsSetting.MountPodMemRequest = volCtx[mountPodMemRequestKey]

		labelString := volCtx[mountPodLabelKey]
		annotationSting := volCtx[mountPodAnnotationKey]
		ctxLabel := make(map[string]string)
		if labelString != "" {
			if err := parseYamlOrJson(labelString, &ctxLabel); err != nil {
				return nil, err
			}
		}
		for k, v := range ctxLabel {
			labels[k] = v
		}
		if annotationSting != "" {
			annos := make(map[string]string)
			if err := parseYamlOrJson(annotationSting, &annos); err != nil {
				return nil, err
			}
			jfsSetting.MountPodAnnotations = annos
		}
	}
	if len(labels) != 0 {
		jfsSetting.MountPodLabels = labels
	}
	return &jfsSetting, nil
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

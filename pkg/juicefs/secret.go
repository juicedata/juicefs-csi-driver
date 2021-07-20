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

package juicefs

import (
	"encoding/json"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type JfsSecret struct {
	IsCe bool

	Name    string `json:"name"`
	MetaUrl string `json:"metaurl"`
	Source  string `json:"source"`

	Configs map[string]string `json:"configs"`
	Envs    map[string]string `json:"envs"`

	MountPodCpuLimit   string `json:"mount_pod_cpu_limit"`
	MountPodMemLimit   string `json:"mount_pod_mem_limit"`
	MountPodCpuRequest string `json:"mount_pod_cpu_request"`
	MountPodMemRequest string `json:"mount_pod_mem_request"`
}

func ParseSecret(secrets, volCtx map[string]string) (*JfsSecret, error) {
	jfsSecret := JfsSecret{}
	if secrets["name"] == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Empty name")
	}
	jfsSecret.Name = secrets["name"]

	m, ok := secrets["metaurl"]
	jfsSecret.MetaUrl = m
	jfsSecret.IsCe = ok

	if secrets["configs"] != "" {
		configStr := secrets["configs"]
		configs := make(map[string]string)
		// json or yaml format
		if err := yaml.Unmarshal([]byte(configStr), &configs); err != nil {
			if err := json.Unmarshal([]byte(configStr), &configs); err != nil {
				return nil, status.Errorf(codes.InvalidArgument,
					"Parse envs in secret error: %v", err)
			}
		}
		jfsSecret.Configs = configs
	}

	if secrets["envs"] != "" {
		envStr := secrets["envs"]
		env := make(map[string]string)
		if err := yaml.Unmarshal([]byte(envStr), &env); err != nil {
			return nil, status.Errorf(codes.InvalidArgument,
				"Parse envs in secret error: %v", err)
		}
		jfsSecret.Envs = env
	}
	if volCtx != nil {
		jfsSecret.MountPodCpuLimit = volCtx["juicefs/mount-cpu-limit"]
		jfsSecret.MountPodMemLimit = volCtx["juicefs/mount-memory-limit"]
		jfsSecret.MountPodCpuRequest = volCtx["juicefs/mount-cpu-request"]
		jfsSecret.MountPodMemRequest = volCtx["juicefs/mount-memory-request"]
	}
	return &jfsSecret, nil
}

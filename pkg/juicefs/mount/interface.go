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

package mount

import (
	"context"

	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
)

type MntInterface interface {
	k8sMount.Interface
	JMount(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error
	JCreateVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error
	JDeleteVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error
	GetMountRef(ctx context.Context, target, podName string) (int, error) // podName is only used by podMount
	UmountTarget(ctx context.Context, target, podName string) error       // podName is only used by podMount
	JUmount(ctx context.Context, target, podName string) error            // podName is only used by podMount
	AddRefOfMount(ctx context.Context, target string, podName string) error
	CleanCache(ctx context.Context, image string, id string, volumeId string, cacheDirs []string) error
}

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

import k8sMount "k8s.io/utils/mount"

type Interface interface {
	k8sMount.Interface
	JMount(volumeId, mountPath string, target string, options []string) error
	JUmount(volumeId, target string) error
	AddRefOfMount(target string, podName string) error
}

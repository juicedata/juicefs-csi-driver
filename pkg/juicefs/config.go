/*

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

var (
	NodeName   = ""
	Namespace  = ""
	MountImage = ""

	MountPodCpuLimit   = "5000m"
	MountPodMemLimit   = "5Gi"
	MountPodCpuRequest = "1000m"
	MountPodMemRequest = "1Gi"

	MountPointPath = "/var/lib/juicefs/volume"
	RootJfsPath    = "/root/.juicefs/jfsmount"
	JFSConfigPath = "/var/lib/juicefs/config"
)

const (
	PodTypeKey    = "app.kubernetes.io/name"
	PodTypeValue  = "juicefs-mount"
	Finalizer     = "juicefs.com/finalizer"
)

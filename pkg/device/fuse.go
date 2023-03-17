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

package device

import (
	"fmt"
	"os"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func getDevices(number int) []*pluginapi.Device {
	hostname, _ := os.Hostname()
	devs := []*pluginapi.Device{}
	for i := 0; i < number; i++ {
		devs = append(devs, &pluginapi.Device{
			ID:     fmt.Sprintf("juicefs-fuse-%s-%d", hostname, i),
			Health: pluginapi.Healthy,
		})
	}
	return devs
}

func deviceExists(devs []*pluginapi.Device, id string) bool {
	for _, d := range devs {
		if d.ID == id {
			return true
		}
	}
	return false
}

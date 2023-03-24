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

package main

import (
	"os"
	"strconv"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/juicedata/juicefs-csi-driver/pkg/device"
)

const defaultMountsAllowed = 5000

func deviceRun() {
	mountsAllowed := defaultMountsAllowed
	if os.Getenv("MOUNTS_ALLOWED") != "" {
		var err error
		if mountsAllowed, err = strconv.Atoi(os.Getenv("MOUNTS_ALLOWED")); err != nil {
			klog.Errorf("got error when parsing MOUNTS_ALLOWED, use default value 5000, err: %v", err)
			mountsAllowed = defaultMountsAllowed
		}
	}
	watcher, err := device.NewFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		klog.Info("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	sigs := device.NewOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	restart := true
	var devicePlugin *device.FuseDevicePlugin

L:
	for {
		if restart {
			if devicePlugin != nil {
				devicePlugin.Stop()
			}

			devicePlugin = device.NewFuseDevicePlugin(mountsAllowed)
			if err := devicePlugin.Serve(); err != nil {
				klog.Info("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate ?")
			} else {
				restart = false
			}
		}

		select {
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				klog.Infof("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				restart = true
			}

		case err := <-watcher.Errors:
			klog.Infof("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				klog.Info("Received SIGHUP, restarting.")
				restart = true
			default:
				klog.Infof("Received signal %v, shutting down.", s)
				devicePlugin.Stop()
				break L
			}
		}
	}
}

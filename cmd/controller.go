/*
 Copyright 2022 Juicedata Inc

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
	"fmt"
	"net/http"
	"os"

	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/juicedata/juicefs-csi-driver/cmd/app"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
)

func parseControllerConfig() {
	config.ByProcess = process
	config.Webhook = webhook
	config.Provisioner = provisioner
	config.FormatInPod = formatInPod
	// enable mount manager by default in csi controller
	config.MountManager = true
	if process {
		// if run in process, does not need pod info
		config.FormatInPod = false
		config.MountManager = false
		config.Webhook = false
		return
	}
	if webhook {
		// if enable webhook, does not need mount manager & must format in pod
		config.FormatInPod = true
		config.MountManager = false
		config.ByProcess = false
	}

	config.NodeName = os.Getenv("NODE_NAME")
	config.Namespace = os.Getenv("JUICEFS_MOUNT_NAMESPACE")
	config.MountPointPath = os.Getenv("JUICEFS_MOUNT_PATH")
	config.JFSConfigPath = os.Getenv("JUICEFS_CONFIG_PATH")
	config.MountLabels = os.Getenv("JUICEFS_MOUNT_LABELS")

	if mountPodImage := os.Getenv("JUICEFS_MOUNT_IMAGE"); mountPodImage != "" {
		config.MountImage = mountPodImage
	}
}

func controllerRun() {
	parseControllerConfig()

	if version {
		info, err := driver.GetVersionJSON()
		if err != nil {
			klog.Fatalln(err)
		}
		fmt.Println(info)
		os.Exit(0)
	}
	if nodeID == "" {
		klog.Fatalln("nodeID must be provided")
	}

	go func() {
		port := 6060
		for {
			http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
			port++
		}
	}()

	// enable mount manager in csi controller
	if config.MountManager {
		go func() {
			mgr, err := app.NewMountManager()
			if err != nil {
				klog.Error(err)
				return
			}
			klog.V(5).Infof("Mount Manager Started")
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				klog.Error(err, "fail to run mount controller")
				return
			}
		}()
	}

	// enable webhook in csi controller
	if config.Webhook {
		go func() {
			klog.V(5).Infof("Webhook Started")
			if err := app.StartWebhook(certDir); err != nil {
				klog.Error(err, "fail to run webhook")
				return
			}
		}()
	}

	drv, err := driver.NewDriver(endpoint, nodeID)
	if err != nil {
		klog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		klog.Fatalln(err)
	}
}

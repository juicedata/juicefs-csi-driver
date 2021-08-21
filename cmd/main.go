/*
Copyright 2018 The Kubernetes Authors.

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
	"flag"
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/cmd/apps"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"k8s.io/klog"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	config.NodeName = os.Getenv("NODE_NAME")
	config.Namespace = os.Getenv("JUICEFS_MOUNT_NAMESPACE")
	config.PodName = os.Getenv("POD_NAME")
	config.MountPointPath = os.Getenv("JUICEFS_MOUNT_PATH")
	config.JFSConfigPath = os.Getenv("JUICEFS_CONFIG_PATH")
	config.JFSMountPriorityName = os.Getenv("JUICEFS_MOUNT_PRIORITY_NAME")
	if config.PodName == "" || config.Namespace == "" {
		klog.Fatalln("Pod name & namespace can't be null.")
		os.Exit(0)
	}
	k8sclient, err := k8s.NewClient()
	if err != nil {
		klog.V(5).Infof("Can't get k8s client: %v", err)
		os.Exit(0)
	}
	pod, err := k8sclient.GetPod(config.PodName, config.Namespace)
	if err != nil {
		klog.V(5).Infof("Can't get pod %s: %v", config.PodName, err)
		os.Exit(0)
	}
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == "juicefs-plugin" {
			config.MountImage = pod.Spec.Containers[i].Image
			return
		}
	}
	klog.V(5).Infof("Can't get container juicefs-plugin in pod %s", config.PodName)
	os.Exit(0)
}

func main() {
	var (
		endpoint      = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
		version       = flag.Bool("version", false, "Print the version and exit.")
		nodeID        = flag.String("nodeid", "", "Node ID")
		enableManager = flag.Bool("enable-manager", false, "Enable manager or not.")
	)
	klog.InitFlags(nil)
	flag.Parse()

	if *version {
		info, err := driver.GetVersionJSON()
		if err != nil {
			klog.Fatalln(err)
		}
		fmt.Println(info)
		os.Exit(0)
	}

	if *nodeID == "" {
		klog.Fatalln("nodeID must be provided")
	}

	if *enableManager {
		manager := apps.NewManager()
		go func() {
			if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
				klog.V(5).Infof("Could not start manager: %v", err)
				os.Exit(1)
			}
		}()
	}

	drv, err := driver.NewDriver(*endpoint, *nodeID)
	if err != nil {
		klog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		klog.Fatalln(err)
	}
}

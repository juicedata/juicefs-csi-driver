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
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"k8s.io/klog"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	endpoint           = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
	version            = flag.Bool("version", false, "Print the version and exit.")
	nodeID             = flag.String("nodeid", "", "Node ID")
	enableManager      = flag.Bool("enable-manager", false, "Enable manager or not.")
	reconcilerInterval = flag.Int("reconciler-interval", 5, "interval (default 5s) for reconciler")
)

func init() {
	klog.InitFlags(nil)
	flag.Parse()
	config.JLock = config.NewPodLock()
	config.NodeName = os.Getenv("NODE_NAME")
	config.Namespace = os.Getenv("JUICEFS_MOUNT_NAMESPACE")
	config.PodName = os.Getenv("POD_NAME")
	config.MountPointPath = os.Getenv("JUICEFS_MOUNT_PATH")
	config.JFSConfigPath = os.Getenv("JUICEFS_CONFIG_PATH")
	config.MountLabels = os.Getenv("JUICEFS_MOUNT_LABELS")
	config.HostIp = os.Getenv("HOST_IP")
	config.KubeletPort = os.Getenv("KUBELET_PORT")
	if config.PodName == "" || config.Namespace == "" {
		klog.Fatalln("Pod name & namespace can't be null.")
		os.Exit(0)
	}

	config.ReconcilerInterval = *reconcilerInterval
	if config.ReconcilerInterval < 5 {
		config.ReconcilerInterval = 5
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
	config.PodServiceAccountName = pod.Spec.ServiceAccountName
	config.CSINodePod = *pod
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
		if config.KubeletPort != "" && config.HostIp != "" {
			if err := controller.StartReconciler(); err != nil {
				klog.V(5).Infof("Could not StartReconciler: %v", err)
				os.Exit(1)
			}
			klog.V(5).Infof("Reconciler Stated")
		} else {
			manager := apps.NewManager()
			go func() {
				if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
					klog.V(5).Infof("Could not start manager: %v", err)
					os.Exit(1)
				}
			}()
		}
	}

	drv, err := driver.NewDriver(*endpoint, *nodeID)
	if err != nil {
		klog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		klog.Fatalln(err)
	}
}

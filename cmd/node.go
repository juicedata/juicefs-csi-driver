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
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var nodeCmd = &cobra.Command{
	Use:   "start-node",
	Short: "juicefs csi node server",
	Run: func(cmd *cobra.Command, args []string) {
		nodeRun()
	},
}

var (
	podManager         bool
	reconcilerInterval int
)

func init() {
	nodeCmd.Flags().BoolVar(&podManager, "enable-manager", false, "Enable pod manager in csi node. default false.")
	nodeCmd.Flags().IntVar(&reconcilerInterval, "reconciler-interval", 5, "interval (default 5s) for reconciler")
}

func parseNodeConfig() {
	config.ByProcess = process
	if process {
		// if run in process, does not need pod info
		config.FormatInPod = false
		return
	}
	config.FormatInPod = formatInPod
	config.NodeName = os.Getenv("NODE_NAME")
	config.Namespace = os.Getenv("JUICEFS_MOUNT_NAMESPACE")
	config.PodName = os.Getenv("POD_NAME")
	config.MountPointPath = os.Getenv("JUICEFS_MOUNT_PATH")
	config.JFSConfigPath = os.Getenv("JUICEFS_CONFIG_PATH")
	config.MountLabels = os.Getenv("JUICEFS_MOUNT_LABELS")
	config.HostIp = os.Getenv("HOST_IP")
	config.KubeletPort = os.Getenv("KUBELET_PORT")
	jfsMountPriorityName := os.Getenv("JUICEFS_MOUNT_PRIORITY_NAME")
	if timeout := os.Getenv("JUICEFS_RECONCILE_TIMEOUT"); timeout != "" {
		duration, _ := time.ParseDuration(timeout)
		if duration > config.ReconcileTimeout {
			config.ReconcileTimeout = duration
		}
	}

	if jfsMountPriorityName != "" {
		config.JFSMountPriorityName = jfsMountPriorityName
	}

	if mountPodImage := os.Getenv("JUICEFS_MOUNT_IMAGE"); mountPodImage != "" {
		config.MountImage = mountPodImage
	}

	if config.PodName == "" || config.Namespace == "" {
		klog.Fatalln("Pod name & namespace can't be null.")
		os.Exit(0)
	}
	config.ReconcilerInterval = reconcilerInterval
	if config.ReconcilerInterval < 5 {
		config.ReconcilerInterval = 5
	}

	k8sclient, err := k8s.NewClient()
	if err != nil {
		klog.V(5).Infof("Can't get k8s client: %v", err)
		os.Exit(0)
	}
	pod, err := k8sclient.GetPod(context.TODO(), config.PodName, config.Namespace)
	if err != nil {
		klog.V(5).Infof("Can't get pod %s: %v", config.PodName, err)
		os.Exit(0)
	}
	config.CSIPod = *pod
}

func nodeRun() {
	parseNodeConfig()

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

	// enable pod manager in csi node
	if !process && podManager && config.KubeletPort != "" && config.HostIp != "" {
		if err := controller.StartReconciler(); err != nil {
			klog.V(5).Infof("Could not Start Reconciler: %v", err)
			os.Exit(1)
		}
		klog.V(5).Infof("Pod Reconciler Started")
	}

	drv, err := driver.NewDriver(endpoint, nodeID)
	if err != nil {
		klog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		klog.Fatalln(err)
	}
}

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
	"strconv"
	"time"

	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/juicedata/juicefs-csi-driver/cmd/app"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func parseNodeConfig() {
	config.ByProcess = process
	if process {
		// if run in process, does not need pod info
		config.FormatInPod = false
		return
	}
	config.FormatInPod = formatInPod

	if jfsImmutable := os.Getenv("JUICEFS_IMMUTABLE"); jfsImmutable != "" {
		if immutable, err := strconv.ParseBool(jfsImmutable); err == nil {
			config.Immutable = immutable
		} else {
			klog.Errorf("cannot parse JUICEFS_IMMUTABLE: %v", err)
		}
	}

	config.NodeName = os.Getenv("NODE_NAME")
	config.Namespace = os.Getenv("JUICEFS_MOUNT_NAMESPACE")
	config.PodName = os.Getenv("POD_NAME")
	config.MountPointPath = os.Getenv("JUICEFS_MOUNT_PATH")
	config.JFSConfigPath = os.Getenv("JUICEFS_CONFIG_PATH")
	config.MountLabels = os.Getenv("JUICEFS_MOUNT_LABELS")

	config.HostIp = os.Getenv("HOST_IP")
	config.KubeletPort = os.Getenv("KUBELET_PORT")
	jfsMountPriorityName := os.Getenv("JUICEFS_MOUNT_PRIORITY_NAME")
	jfsMountPreemptionPolicy := os.Getenv("JUICEFS_MOUNT_PREEMPTION_POLICY")
	if timeout := os.Getenv("JUICEFS_RECONCILE_TIMEOUT"); timeout != "" {
		duration, _ := time.ParseDuration(timeout)
		if duration > config.ReconcileTimeout {
			config.ReconcileTimeout = duration
		}
	}

	if jfsMountPriorityName != "" {
		config.JFSMountPriorityName = jfsMountPriorityName
	}

	if jfsMountPreemptionPolicy != "" {
		config.JFSMountPreemptionPolicy = jfsMountPreemptionPolicy
	}

	if mountPodImage := os.Getenv("JUICEFS_CE_MOUNT_IMAGE"); mountPodImage != "" {
		config.CEMountImage = mountPodImage
	}
	if mountPodImage := os.Getenv("JUICEFS_EE_MOUNT_IMAGE"); mountPodImage != "" {
		config.EEMountImage = mountPodImage
	}
	if mountPodImage := os.Getenv("JUICEFS_MOUNT_IMAGE"); mountPodImage != "" {
		// check if it's CE or EE
		hasCE, hasEE := util.ImageResol(mountPodImage)
		if hasCE {
			config.CEMountImage = mountPodImage
		}
		if hasEE {
			config.EEMountImage = mountPodImage
		}
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
	if !process && podManager {
		needStartPodManager := false
		if config.KubeletPort != "" && config.HostIp != "" {
			if err := controller.StartReconciler(); err != nil {
				klog.V(5).Info("Could not Start Reconciler of polling kubelet and fallback to watch ApiServer.")
				needStartPodManager = true
			}
		} else {
			needStartPodManager = true
		}

		if needStartPodManager {
			go func() {
				ctx := ctrl.SetupSignalHandler()
				mgr, err := app.NewPodManager()
				if err != nil {
					klog.Fatalln(err)
				}

				if err := mgr.Start(ctx); err != nil {
					klog.Fatalln(err)
				}
			}()
		}
		klog.V(5).Infof("Pod Reconciler Started")
	}

	drv, err := driver.NewDriver(endpoint, nodeID, leaderElection, leaderElectionNamespace, leaderElectionLeaseDuration)
	if err != nil {
		klog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		klog.Fatalln(err)
	}
}

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
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"os"

	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	"k8s.io/klog"
)

func init() {
	juicefs.NodeName = os.Getenv("NODE_NAME")
	juicefs.Namespace = os.Getenv("MOUNT_NAMESPACE")
	juicefs.MountImage = os.Getenv("MOUNT_IMAGE")
	juicefs.MountPointPath = os.Getenv("JUICEFS_MOUNT_PATH")
	juicefs.MountPodCpuLimit = os.Getenv("JUICEFS_MOUNT_POD_CPU_LIMIT")
	juicefs.MountPodMemLimit = os.Getenv("JUICEFS_MOUNT_POD_MEM_LIMIT")
	juicefs.MountPodCpuRequest = os.Getenv("JUICEFS_MOUNT_POD_CPU_REQUEST")
	juicefs.MountPodMemRequest = os.Getenv("JUICEFS_MOUNT_POD_MEM_REQUEST")
}

func main() {
	var (
		endpoint = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
		version  = flag.Bool("version", false, "Print the version and exit.")
		nodeID   = flag.String("nodeid", "", "Node ID")
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

	go func() {
		apps.Run()
	}()

	drv, err := driver.NewDriver(*endpoint, *nodeID)
	if err != nil {
		klog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		klog.Fatalln(err)
	}
}

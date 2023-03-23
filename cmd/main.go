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
	goflag "flag"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/klog"
)

var (
	endpoint    string
	version     bool
	nodeID      string
	formatInPod bool
	process     bool

	provisioner bool
	webhook     bool
	certDir     string
	webhookPort int

	podManager         bool
	reconcilerInterval int
	devicePlugin       bool
	mountsAllowed      = 5000
)

func main() {
	var cmd = &cobra.Command{
		Use:   "juicefs-csi",
		Short: "juicefs csi driver",
		Run: func(cmd *cobra.Command, args []string) {
			run()
		},
	}
	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	cmd.PersistentFlags().BoolVar(&version, "version", false, "Print the version and exit.")
	cmd.PersistentFlags().StringVar(&nodeID, "nodeid", "", "Node ID")
	cmd.PersistentFlags().BoolVar(&formatInPod, "format-in-pod", false, "Put format/auth in pod")
	cmd.PersistentFlags().BoolVar(&process, "by-process", false, "CSI Driver run juicefs in process or not. default false.")

	// controller flags
	cmd.Flags().BoolVar(&provisioner, "provisioner", false, "Enable provisioner in controller. default false.")
	cmd.Flags().BoolVar(&webhook, "webhook", false, "Enable webhook in controller. default false.")
	cmd.Flags().StringVar(&certDir, "webhook-cert-dir", "/etc/webhook/certs", "Admission webhook cert/key dir.")
	cmd.Flags().IntVar(&webhookPort, "webhook-port", 9444, "Admission webhook cert/key dir.")

	// node flags
	cmd.Flags().BoolVar(&podManager, "enable-manager", false, "Enable pod manager in csi node. default false.")
	cmd.Flags().IntVar(&reconcilerInterval, "reconciler-interval", 5, "interval (default 5s) for reconciler")
	cmd.Flags().BoolVar(&devicePlugin, "enable-device", false, "Enable fuse device plugin in csi node. default false.")
	cmd.Flags().IntVar(&mountsAllowed, "mounts-allowed", 5000, "maximum times the fuse device can be mounted")

	goFlag := goflag.CommandLine
	klog.InitFlags(goFlag)
	cmd.PersistentFlags().AddGoFlagSet(goFlag)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run() {
	podName := os.Getenv("POD_NAME")
	if strings.Contains(podName, "csi-controller") {
		klog.Info("Run CSI controller")
		controllerRun()
	}
	if strings.Contains(podName, "csi-node") {
		if devicePlugin {
			klog.Info("Run fuse device plugin")
			go deviceRun()
		}
		klog.Info("Run CSI node")
		nodeRun()
	}
}

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
	"fmt"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	"github.com/spf13/cobra"
	"k8s.io/klog"
)

var (
	endpoint    string
	version     bool
	nodeID      string
	formatInPod bool
	process     bool

	provisioner       bool
	cacheConf         bool
	webhook           bool
	certDir           string
	webhookPort       int
	validationWebhook bool

	podManager         bool
	reconcilerInterval int

	leaderElection              bool
	leaderElectionNamespace     string
	leaderElectionLeaseDuration time.Duration
)

func main() {
	var cmd = &cobra.Command{
		Use:   "juicefs-csi",
		Short: "juicefs csi driver",
		Run: func(cmd *cobra.Command, args []string) {
			if version {
				info, err := driver.GetVersionJSON()
				if err != nil {
					klog.Fatalln(err)
				}
				fmt.Println(info)
				os.Exit(0)
			}

			run()
		},
	}
	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	cmd.PersistentFlags().BoolVar(&version, "version", false, "Print the version and exit.")
	cmd.PersistentFlags().StringVar(&nodeID, "nodeid", "", "Node ID")
	cmd.PersistentFlags().BoolVar(&formatInPod, "format-in-pod", false, "Put format/auth in pod")
	cmd.PersistentFlags().BoolVar(&process, "by-process", false, "CSI Driver run juicefs in process or not. default false.")

	cmd.PersistentFlags().BoolVar(&leaderElection, "leader-election", false, "Enables leader election. If leader election is enabled, additional RBAC rules are required. ")
	cmd.PersistentFlags().StringVar(&leaderElectionNamespace, "leader-election-namespace", "", "Namespace where the leader election resource lives. Defaults to the pod namespace if not set.")
	cmd.PersistentFlags().DurationVar(&leaderElectionLeaseDuration, "leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership. Defaults to 15 seconds.")

	// controller flags
	cmd.Flags().BoolVar(&provisioner, "provisioner", false, "Enable provisioner in controller. default false.")
	cmd.Flags().BoolVar(&cacheConf, "cache-client-conf", false, "Cache client config file. default false.")
	cmd.Flags().BoolVar(&webhook, "webhook", false, "Enable mutating webhook in controller for sidecar mode. default false.")
	cmd.Flags().StringVar(&certDir, "webhook-cert-dir", "/etc/webhook/certs", "Admission webhook cert/key dir.")
	cmd.Flags().IntVar(&webhookPort, "webhook-port", 9444, "Admission webhook port.")
	cmd.Flags().BoolVar(&validationWebhook, "validating-webhook", false, "Enable validation webhook in controller. default false.")

	// node flags
	cmd.Flags().BoolVar(&podManager, "enable-manager", false, "Enable pod manager in csi node. default false.")
	cmd.Flags().IntVar(&reconcilerInterval, "reconciler-interval", 5, "interval (default 5s) for reconciler")

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
		klog.Info("Run CSI node")
		nodeRun()
	}
}

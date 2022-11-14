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

	"github.com/spf13/cobra"
	"k8s.io/klog"
)

var (
	endpoint    string
	version     bool
	nodeID      string
	formatInPod bool
	process     bool
)

func main() {
	var cmd = &cobra.Command{
		Use:   "juicefs-csi-driver",
		Short: "juicefs csi driver",
	}

	// parse global flags
	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	cmd.PersistentFlags().BoolVar(&version, "version", false, "Print the version and exit.")
	cmd.PersistentFlags().StringVar(&nodeID, "nodeid", "", "Node ID")
	cmd.PersistentFlags().BoolVar(&formatInPod, "format-in-pod", false, "Put format/auth in pod")
	cmd.PersistentFlags().BoolVar(&process, "by-process", false, "CSI Driver run juicefs in process or not. default false.")
	goFlag := goflag.CommandLine
	klog.InitFlags(goFlag)
	cmd.PersistentFlags().AddGoFlagSet(goFlag)

	// add sub commands
	controllerCmd := NewControllerCmd()
	nodeCmd := NewNodeCmd()
	cmd.AddCommand(controllerCmd, nodeCmd)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

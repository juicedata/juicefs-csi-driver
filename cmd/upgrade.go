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
	"strings"

	"github.com/spf13/cobra"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/grace"
)

var (
	recreate    = false
	worker      = 1
	ignoreError = false
	uniqueId    = ""
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "upgrade mount pod smoothly",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			log.Info("please specify the name of the mount pod which you want to upgrade", "node", config.NodeName)
			os.Exit(1)
		}
		name := args[0]
		if strings.ToLower(name) == "batch" {
			if err := grace.TriggerBatchUpgrade(config.ShutdownSockPath, recreate, worker, ignoreError, uniqueId); err != nil {
				log.Error(err, "failed to upgrade mount pod")
				os.Exit(1)
			}
		} else {
			if err := grace.TriggerShutdown(config.ShutdownSockPath, name, recreate); err != nil {
				log.Error(err, "failed to upgrade mount pod")
				os.Exit(1)
			}
		}
	},
}

func init() {
	upgradeCmd.Flags().BoolVar(&recreate, "recreate", false, "smoothly upgrade the mount pod with recreate")
	upgradeCmd.Flags().BoolVar(&ignoreError, "ignoreError", false, "ignore error and upgrade the rest mount pods")
	upgradeCmd.Flags().IntVar(&worker, "worker", 1, "worker number for batch upgrade")
	upgradeCmd.Flags().StringVar(&uniqueId, "uniqueId", "", "unique id for batch upgrade")
}

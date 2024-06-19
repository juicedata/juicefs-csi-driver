/*
 Copyright 2024 Juicedata Inc

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

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var KubernetesConfigFlags *genericclioptions.ConfigFlags

const version = "v1.0.0"

func init() {
	KubernetesConfigFlags = genericclioptions.NewConfigFlags(true)
	rootCmd.PersistentFlags().BoolP("json", "j", false, "show json format")
	KubernetesConfigFlags.AddFlags(rootCmd.PersistentFlags())
}

var rootCmd = &cobra.Command{
	Use:     "kubectl-jfs",
	Short:   "Root short description",
	Long:    "Root long description",
	Version: version,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf(`
Usage:
  kubectl-jfs [flags]

Flags:
  pod, po 		   show app pods using juicefs pvc
  mount 		   show mount pod of juicefs

  -j, --json           show json format
  -n, --namespace      namespace of resource, default is default 
`)
		return nil
	},
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}

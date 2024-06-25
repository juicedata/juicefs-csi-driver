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
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var debugCmd = &cobra.Command{
	Use:                   "debug <resource> <name>",
	Short:                 "Debug the pod/pv/pvc which is using juicefs",
	DisableFlagsInUseLine: true,
	Example: `  # debug the pod which is using juicefs pvc
  kubectl juicefs debug po <pod-name> -n <namespace>

  # debug pvc using juicefs pv
  kubectl juicefs debug pvc <pvc-name> -n <namespace>

  # debug pv which is juicefs pv
  kubectl juicefs debug pv <pv-name> 
`,
	RunE: debug,
}

func init() {
	rootCmd.AddCommand(debugCmd)
}

func debug(cmd *cobra.Command, args []string) error {
	clientSet := ClientSet(KubernetesConfigFlags)
	if len(args) != 2 {
		return fmt.Errorf("please specify the resource")
	}
	resourceType := args[0]
	resourceName := args[1]
	ns, _ := rootCmd.Flags().GetString("namespace")
	if ns == "" {
		ns = "default"
	}

	var (
		out string
		err error
	)

	switch resourceType {
	case "po":
		fallthrough
	case "pod":
		var (
			pod      *corev1.Pod
			describe *podDescribe
		)
		if pod, err = clientSet.CoreV1().Pods(ns).Get(context.Background(), resourceName, metav1.GetOptions{}); err != nil {
			return err
		}
		describe, err = newPodDescribe(clientSet, pod)
		if err != nil {
			return err
		}
		out, err = describe.debug().describePod()
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	return nil
}

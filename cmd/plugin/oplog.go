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
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var oplogCmd = &cobra.Command{
	Use:                   "oplog <name>",
	Short:                 "collect access log from mount pod",
	DisableFlagsInUseLine: true,
	Example: `  # collect access log from mount pod
  kubectl jfs oplog <pod-name>
`,
	RunE: oplog,
}

func init() {
	rootCmd.AddCommand(oplogCmd)
}

func oplog(cmd *cobra.Command, args []string) (err error) {
	clientSet := ClientSet(KubernetesConfigFlags)
	eCli := NewExecCli(clientSet)

	cmd.Flags().BoolVarP(&eCli.Stdin, "stdin", "i", eCli.Stdin, "Pass stdin to the container")
	cmd.Flags().BoolVarP(&eCli.TTY, "tty", "t", eCli.TTY, "Stdin is a TTY")
	cmd.Flags().BoolVarP(&eCli.Quiet, "quiet", "q", eCli.Quiet, "Only print output from the remote session")

	if len(args) < 1 {
		return fmt.Errorf("please specify the mount pod name")
	}

	podName := args[0]
	if !strings.HasPrefix(podName, "juicefs-") {
		fmt.Printf("pod %s is not juicefs mount pod\n", podName)
		return nil
	}
	var pod *corev1.Pod

	if pod, err = clientSet.CoreV1().Pods("kube-system").Get(context.Background(), podName, metav1.GetOptions{}); err != nil {
		fmt.Printf("get mount pod %s error: %s\n", podName, err.Error())
		return nil
	}
	if pod.Labels[config.PodTypeKey] != config.PodTypeValue {
		fmt.Println(fmt.Errorf("pod %s is not juicefs mount pod", podName))
		return nil
	}

	mountPath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		fmt.Printf("get mount path of pod %s error: %s\n", podName, err.Error())
		return nil
	}
	return eCli.Completion().
		SetNamespace("kube-system").
		SetPod(podName).
		Container(config.MountContainerName).
		Commands([]string{"cat", fmt.Sprintf("%s/.accesslog", mountPath)}).
		Run()
}

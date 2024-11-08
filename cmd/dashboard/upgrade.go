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
	"os"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	node     string
	recreate = false
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "trigger upgrade mount pod smoothly",
	Run: func(cmd *cobra.Command, args []string) {
		var config *rest.Config
		var err error
		var sysNamespace string
		var csiNodes []corev1.Pod
		sysNamespace = os.Getenv(SysNamespaceKey)
		if sysNamespace == "" {
			sysNamespace = "kube-system"
		}
		if devMode {
			config, _ = getLocalConfig()
		} else {
			gin.SetMode(gin.ReleaseMode)
			config = ctrl.GetConfigOrDie()
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Error(err, "Failed to create kubernetes clientset")
			os.Exit(1)
		}
		s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": "juicefs-csi-driver",
				"app":                    "juicefs-csi-node",
			},
		})

		var csiList *corev1.PodList
		if node != "" {
			f := &fields.Set{"spec.nodeName": node}
			csiList, err = clientset.CoreV1().Pods(sysNamespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: s.String(),
				FieldSelector: f.String(),
			})
		} else {
			csiList, err = clientset.CoreV1().Pods(sysNamespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: s.String(),
			})
		}
		if err != nil {
			log.Error(err, "Failed to list csi nodes")
			os.Exit(1)
		}
		csiNodes = csiList.Items

		for _, csiNode := range csiNodes {
			log.Info(fmt.Sprintf("Start to upgrade mount pods on node %s", csiNode.Spec.NodeName))
			if err = triggerUpgradeInNode(clientset, config, csiNode); err != nil {
				log.Error(err, "Failed to upgrade mount pods", "node", node)
				os.Exit(1)
			}
		}
	},
}

func init() {
	upgradeCmd.Flags().StringVar(&node, "node", "", "upgrade all the mount pods on node")
	upgradeCmd.Flags().BoolVar(&recreate, "recreate", false, "upgrade the mount pod with recreate")
}

func triggerUpgradeInNode(client kubernetes.Interface, cfg *rest.Config, csiNode corev1.Pod) error {
	cmds := []string{"juicefs-csi-driver", "upgrade", "BATCH"}
	if recreate {
		cmds = append(cmds, "--recreate")
	}
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(csiNode.Name).
		Namespace(csiNode.Namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   cmds,
		Container: "juicefs-plugin",
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, k8scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		log.Error(err, "Failed to create SPDY executor")
		return err
	}
	if err := executor.Stream(remotecommand.StreamOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    true,
	}); err != nil {
		return err
	}
	return nil
}

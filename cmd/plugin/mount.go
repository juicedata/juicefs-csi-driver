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
	"io"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	kdescribe "k8s.io/kubectl/pkg/describe"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

var mountCmd = &cobra.Command{
	Use:   "mount",
	Short: "Show mount pod of juicefs",
	RunE:  listMountPod,
}

type mountPod struct {
	namespace string
	name      string
	appPods   []string
	node      string
	csiNode   string
	status    string
}

func init() {
	rootCmd.AddCommand(mountCmd)
}

func listMountPod(cmd *cobra.Command, args []string) error {
	clientSet := ClientSet(KubernetesConfigFlags)

	mountList, err := GetMountPodList(clientSet)
	if err != nil {
		return err
	}

	csiNodeList, err := GetCSINodeList(clientSet)
	if err != nil {
		return err
	}

	nsList, err := GetNamespaceList(clientSet)
	if err != nil {
		return err
	}

	appList := make([]corev1.Pod, 0)
	for _, ns := range nsList.Items {
		podList, err := GetAppPodList(clientSet, ns.Name)
		if err != nil {
			return err
		}
		appList = append(appList, podList.Items...)
	}

	mountMapList := map[string]map[string]string{}
	for _, mount := range mountList.Items {
		if _, ok := mountMapList[mount.Spec.NodeName]; !ok {
			mountMapList[mount.Spec.NodeName] = make(map[string]string)
		}
		mountMapList[mount.Spec.NodeName][mount.Annotations[config.UniqueId]] = mount.Name
	}

	csiNodeMapList := map[string]string{}
	for _, csi := range csiNodeList.Items {
		csiNodeMapList[csi.Spec.NodeName] = csi.Name
	}

	appMapList := map[string]string{}
	for _, po := range appList {
		appMapList[string(po.UID)] = fmt.Sprintf("%s/%s", po.Namespace, po.Name)
	}

	mountPods := make([]mountPod, 0, len(mountList.Items))
	t := mountList.Items
	for i := 0; i < len(t); i++ {
		pod := t[i]
		mount := mountPod{
			namespace: pod.Namespace,
			name:      pod.Name,
			node:      pod.Spec.NodeName,
		}

		appNames := []string{}
		for uid, app := range appMapList {
			for _, v := range pod.Annotations {
				if strings.Contains(v, uid) {
					appNames = append(appNames, app)
				}
			}
		}
		mount.appPods = appNames
		mount.csiNode = csiNodeMapList[pod.Spec.NodeName]
		mount.status = string(pod.Status.Phase)
		mountPods = append(mountPods, mount)
	}

	if len(mountPods) == 0 {
		fmt.Printf("No mount pod found in %s namespace.", "kube-system")
		return nil
	}

	out, err := printMountPods(mountPods)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	return nil
}

func printMountPods(pods []mountPod) (string, error) {
	return tabbedString(func(out io.Writer) error {
		w := kdescribe.NewPrefixWriter(out)
		w.Write(kdescribe.LEVEL_0, "Name\tNamespace\tApp Pods\tStatus\tCSI Node\tNode\n")
		for _, pod := range pods {
			w.Write(kdescribe.LEVEL_0, "%s\t%s\t%s\t%s\t%s\t%s\n", pod.name, pod.namespace, strings.Join(pod.appPods, ","), pod.status, pod.csiNode, pod.node)
		}
		return nil
	})
}

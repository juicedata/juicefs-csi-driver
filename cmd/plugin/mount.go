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
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

var mountCmd = &cobra.Command{
	Use:   "mount",
	Short: "show mount pod of juicefs",
	RunE:  mountPod,
}

var MountTitle = []string{
	"NAMESPACE",
	"NAME",
	"APP_POD",
	"NODE",
	"CSI_NODE",
}

func init() {
	rootCmd.AddCommand(mountCmd)
}

func mountPod(cmd *cobra.Command, args []string) error {
	clientSet := ClientSet(KubernetesConfigFlags)

	mountList, err := GetMountPods(clientSet)
	if err != nil {
		return err
	}

	csiNodeList, err := GetCSINode(clientSet)
	if err != nil {
		return err
	}

	nsList, err := GetNamespaces(clientSet)
	if err != nil {
		return err
	}

	appList := make([]corev1.Pod, 0)
	for _, ns := range nsList.Items {
		podList, err := GetAppPods(clientSet, ns.Name)
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

	podMapList := []map[string]string{}
	t := mountList.Items
	for i := 0; i < len(t); i++ {
		pod := t[i]
		poMap := make(map[string]string)
		poMap["NAMESPACE"] = "kube-system"
		poMap["NAME"] = pod.Name
		poMap["NODE"] = pod.Spec.NodeName

		appNames := []string{}
		for uid, app := range appMapList {
			for _, v := range pod.Annotations {
				if strings.Contains(v, uid) {
					appNames = append(appNames, app)
				}
			}
		}
		poMap["APP_POD"] = strings.Join(appNames, ",")
		poMap["CSI_NODE"] = csiNodeMapList[pod.Spec.NodeName]
		podMapList = append(podMapList, poMap)
	}

	if len(podMapList) == 0 {
		fmt.Printf("No mount pod found in %s namespace.", "kube-system")
		return nil
	}

	// gen t
	tb := GenTable(MountTitle, podMapList)
	// json format
	json, _ := rootCmd.Flags().GetBool("json")
	if json {
		jsonStr, _ := tb.JSON(2)
		fmt.Println(jsonStr)
		return nil
	}
	fmt.Println(tb.String())
	return nil
}

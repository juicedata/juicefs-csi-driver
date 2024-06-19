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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

var podCmd = &cobra.Command{
	Use:     "pod",
	Aliases: []string{"po"},
	Short:   "show pods using juicefs pvc",
	RunE:    jfsPod,
}

var PodTitle = []string{
	"NAMESPACE",
	"NAME",
	"PVC",
	"PV",
	"STORAGECLASS",
	"MOUNT_POD",
	"NODE",
	"CSI_NODE",
}

func init() {
	rootCmd.AddCommand(podCmd)
}

func jfsPod(cmd *cobra.Command, args []string) error {
	clientSet := ClientSet(KubernetesConfigFlags)
	ns, _ := rootCmd.Flags().GetString("namespace")
	if ns == "" {
		ns = "default"
	}

	podList, err := GetAppPods(clientSet, ns)
	if err != nil {
		return err
	}

	mountList, err := GetMountPods(clientSet)
	if err != nil {
		return err
	}

	csiNodeList, err := GetCSINode(clientSet)
	if err != nil {
		return err
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

	podMapList := []map[string]string{}
	t := podList.Items
	for i := 0; i < len(t); i++ {
		pod := t[i]
		poMap := make(map[string]string)

		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcName := volume.PersistentVolumeClaim.ClaimName
				pvc, err := clientSet.CoreV1().PersistentVolumeClaims(ns).Get(context.Background(), pvcName, metav1.GetOptions{})
				if err != nil {
					fmt.Printf("get pvc error: %s", err.Error())
					return err
				}
				if pvc.Spec.VolumeName != "" {
					pv, err := clientSet.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
					if err != nil {
						fmt.Printf("get pv error: %s", err.Error())
						return err
					}
					if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
						poMap["NAMESPACE"] = ns
						poMap["NAME"] = pod.Name
						poMap["PVC"] = pvc.Name
						poMap["PV"] = pv.Name
						poMap["STORAGECLASS"] = pv.Spec.StorageClassName
						poMap["NODE"] = pod.Spec.NodeName
						poMap["MOUNT_POD"] = mountMapList[pod.Spec.NodeName][pv.Spec.CSI.VolumeHandle]
						poMap["CSI_NODE"] = csiNodeMapList[pod.Spec.NodeName]
						podMapList = append(podMapList, poMap)
					}
				}
			}
		}
	}

	if len(podMapList) == 0 {
		fmt.Printf("No pod found using juicefs PVC in %s namespace.", ns)
		return nil
	}

	// gen t
	tb := GenTable(PodTitle, podMapList)
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

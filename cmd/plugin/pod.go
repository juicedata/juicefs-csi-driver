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
	"io"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kdescribe "k8s.io/kubectl/pkg/describe"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

var podCmd = &cobra.Command{
	Use:     "pod",
	Aliases: []string{"po"},
	Short:   "Show pods using juicefs pvc",
	RunE:    jfsPod,
}

type appPod struct {
	namespace string
	name      string
	mountPods []string
	node      string
	csiNode   string
	status    string
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

	podList, err := GetPodList(clientSet, ns)
	if err != nil {
		return err
	}

	mountList, err := GetMountPodList(clientSet)
	if err != nil {
		return err
	}

	csiNodeList, err := GetCSINodeList(clientSet)
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

	appPods := make([]appPod, 0, len(podList.Items))
	t := podList.Items
	for i := 0; i < len(t); i++ {
		pod := t[i]

		if len(pod.Spec.Volumes) == 0 {
			continue
		}

		appending := false
		po := appPod{
			namespace: pod.Namespace,
			name:      pod.Name,
			node:      pod.Spec.NodeName,
			csiNode:   csiNodeMapList[pod.Spec.NodeName],
			status:    string(pod.Status.Phase),
		}
		mounts := make([]string, 0)
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcName := volume.PersistentVolumeClaim.ClaimName
				pvc, err := clientSet.CoreV1().PersistentVolumeClaims(ns).Get(context.Background(), pvcName, metav1.GetOptions{})
				if err != nil {
					if !k8serrors.IsNotFound(err) {
						fmt.Printf("get pvc error: %s", err.Error())
						return err
					}
					appending = true
				}
				if pvc.Status.Phase != corev1.ClaimBound {
					appending = true
				}
				if pvc.Spec.VolumeName != "" {
					pv, err := clientSet.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
					if err != nil {
						fmt.Printf("get pv error: %s", err.Error())
						return err
					}
					if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
						appending = true
						mounts = append(mounts, mountMapList[pod.Spec.NodeName][pv.Spec.CSI.VolumeHandle])
					}
				}
			}
		}
		po.mountPods = mounts
		if appending {
			appPods = append(appPods, po)
		}
	}

	if len(appPods) == 0 {
		fmt.Printf("No pod found using juicefs PVC in %s namespace.", ns)
		return nil
	}

	out, err := printAppPods(appPods)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	return nil
}

func printAppPods(pods []appPod) (string, error) {
	return tabbedString(func(out io.Writer) error {
		w := kdescribe.NewPrefixWriter(out)
		w.Write(kdescribe.LEVEL_0, "Name\tNamespace\tMount Pods\tStatus\tCSI Node\tNode\n")
		for _, pod := range pods {
			w.Write(kdescribe.LEVEL_0, "%s\t%s\t%s\t%s\t%s\t%s\n",
				pod.name,
				pod.namespace,
				strings.Join(pod.mountPods, ","),
				pod.status,
				pod.csiNode,
				pod.node,
			)
		}
		return nil
	})
}

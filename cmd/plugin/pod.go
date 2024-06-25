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
	"k8s.io/client-go/kubernetes"
	kdescribe "k8s.io/kubectl/pkg/describe"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

var podCmd = &cobra.Command{
	Use:     "pod",
	Aliases: []string{"po"},
	Short:   "Show pods using juicefs pvc",
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := rootCmd.Flags().GetString("namespace")
		if ns == "" {
			ns = "default"
		}
		aa, err := newAppAnalyzer(ns)
		if err != nil {
			return err
		}
		return aa.jfsPod(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(podCmd)
}

type appAnalyzer struct {
	clientSet *kubernetes.Clientset
	ns        string
	pods      []corev1.Pod
	mountPods []corev1.Pod
	csiNodes  map[string]string
	pvcs      map[string]corev1.PersistentVolumeClaim
	pvs       map[string]corev1.PersistentVolume

	apps []appPod
}

func newAppAnalyzer(ns string) (aa *appAnalyzer, err error) {
	clientSet := ClientSet(KubernetesConfigFlags)
	aa = &appAnalyzer{
		clientSet: clientSet,
		ns:        ns,
		pods:      make([]corev1.Pod, 0),
		mountPods: make([]corev1.Pod, 0),
		csiNodes:  map[string]string{},
		pvcs:      map[string]corev1.PersistentVolumeClaim{},
		pvs:       map[string]corev1.PersistentVolume{},
		apps:      make([]appPod, 0),
	}
	var (
		csiNodeList = make([]corev1.Pod, 0)
		pvcList     = make([]corev1.PersistentVolumeClaim, 0)
		pvList      = make([]corev1.PersistentVolume, 0)
	)
	if aa.pods, err = GetPodList(clientSet, ns); err != nil {
		return
	}
	if aa.mountPods, err = GetMountPodList(clientSet); err != nil {
		return
	}
	if csiNodeList, err = GetCSINodeList(clientSet); err != nil {
		return
	}
	for _, csi := range csiNodeList {
		aa.csiNodes[csi.Spec.NodeName] = csi.Name
	}
	if pvcList, err = GetPVCList(clientSet, ns); err != nil {
		return
	}
	for _, pvc := range pvcList {
		aa.pvcs[pvc.Name] = pvc
	}
	if pvList, err = GetPVList(clientSet); err != nil {
		return
	}
	for _, pv := range pvList {
		aa.pvs[pv.Name] = pv
	}

	return
}

type appPod struct {
	namespace string
	name      string
	mountPods []string
	node      string
	csiNode   string
	status    string
}

func (aa *appAnalyzer) jfsPod(cmd *cobra.Command, args []string) error {
	appPods := make([]appPod, 0, len(aa.pods))
	for i := 0; i < len(aa.pods); i++ {
		pod := aa.pods[i]

		if len(pod.Spec.Volumes) == 0 {
			continue
		}

		appending := false
		po := appPod{
			namespace: pod.Namespace,
			name:      pod.Name,
			node:      pod.Spec.NodeName,
			csiNode:   aa.csiNodes[pod.Spec.NodeName],
			status:    getPodStatus(pod),
		}
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcName := volume.PersistentVolumeClaim.ClaimName
				pvc, ok := aa.pvcs[pvcName]
				if !ok {
					appending = true
					continue
				}
				if pvc.Status.Phase != corev1.ClaimBound {
					appending = true
					continue
				}
				if pvc.Spec.VolumeName != "" {
					pv, ok := aa.pvs[pvc.Spec.VolumeName]
					if !ok {
						appending = true
						continue
					}
					if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
						appending = true
					}
				}
			}
		}
		for _, mount := range aa.mountPods {
			for _, value := range mount.Annotations {
				if strings.Contains(value, string(pod.UID)) {
					po.mountPods = append(po.mountPods, mount.Name)
					appending = true
				}
			}
		}
		if appending {
			appPods = append(appPods, po)
		}
	}

	if len(appPods) == 0 {
		fmt.Printf("No pod found using juicefs PVC in %s namespace.", aa.ns)
		return nil
	}

	aa.apps = appPods
	out, err := aa.printAppPods()
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	return nil
}

func (aa *appAnalyzer) printAppPods() (string, error) {
	return tabbedString(func(out io.Writer) error {
		w := kdescribe.NewPrefixWriter(out)
		w.Write(kdescribe.LEVEL_0, "Name\tNamespace\tMount Pods\tStatus\tCSI Node\tNode\n")
		for _, pod := range aa.apps {
			for i, mount := range pod.mountPods {
				name, namespace, status, csiNode, node := "", "", "", "", ""
				mountShow := mount
				if i < len(pod.mountPods)-1 {
					mountShow = mount + ","
				}
				if i == 0 {
					name, namespace, status, csiNode, node = ifNil(pod.name), ifNil(pod.namespace), ifNil(pod.status), ifNil(pod.csiNode), ifNil(pod.node)
				}
				w.Write(kdescribe.LEVEL_0, "%s\t%s\t%s\t%s\t%s\t%s\n", name, namespace, mountShow, status, csiNode, node)
			}
			if len(pod.mountPods) == 0 {
				w.Write(kdescribe.LEVEL_0, "%s\t%s\t%s\t%s\t%s\t%s\n", ifNil(pod.name), ifNil(pod.namespace), "<none>", ifNil(pod.status), ifNil(pod.csiNode), ifNil(pod.node))
			}
		}
		return nil
	})
}

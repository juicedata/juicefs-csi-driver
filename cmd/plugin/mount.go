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
)

var mountCmd = &cobra.Command{
	Use:   "mount",
	Short: "Show mount pod of juicefs",
	RunE: func(cmd *cobra.Command, args []string) error {
		ma, err := newMountAnalyzer()
		if err != nil {
			return err
		}
		return ma.listMountPod(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(mountCmd)
}

type mountAnalyzer struct {
	clientSet *kubernetes.Clientset
	apps      map[string]string
	mountPods []corev1.Pod
	csiNodes  map[string]string
	pvcs      map[string]corev1.PersistentVolumeClaim
	pvs       map[string]corev1.PersistentVolume

	mounts []mountPod
}

func newMountAnalyzer() (ma *mountAnalyzer, err error) {
	clientSet := ClientSet(KubernetesConfigFlags)
	ma = &mountAnalyzer{
		clientSet: clientSet,
		apps:      make(map[string]string),
		mountPods: make([]corev1.Pod, 0),
		csiNodes:  map[string]string{},
		pvcs:      map[string]corev1.PersistentVolumeClaim{},
		pvs:       map[string]corev1.PersistentVolume{},
		mounts:    make([]mountPod, 0),
	}
	var (
		nsList      []corev1.Namespace
		podList     []corev1.Pod
		csiNodeList = make([]corev1.Pod, 0)
	)
	if nsList, err = GetNamespaceList(clientSet); err != nil {
		return
	}
	for _, ns := range nsList {
		podList, err = GetAppPodList(clientSet, ns.Name)
		if err != nil {
			return
		}
		for _, po := range podList {
			ma.apps[string(po.UID)] = fmt.Sprintf("%s/%s", po.Namespace, po.Name)
		}
	}

	if ma.mountPods, err = GetMountPodList(ma.clientSet); err != nil {
		return
	}

	if csiNodeList, err = GetCSINodeList(ma.clientSet); err != nil {
		return
	}
	for _, csi := range csiNodeList {
		ma.csiNodes[csi.Spec.NodeName] = csi.Name
	}
	return
}

type mountPod struct {
	namespace string
	name      string
	appPods   []string
	node      string
	csiNode   string
	status    string
}

func (ma *mountAnalyzer) listMountPod(cmd *cobra.Command, args []string) error {
	for i := 0; i < len(ma.mountPods); i++ {
		pod := ma.mountPods[i]
		mount := mountPod{
			namespace: pod.Namespace,
			name:      pod.Name,
			node:      pod.Spec.NodeName,
		}

		appNames := []string{}
		for uid, app := range ma.apps {
			for _, v := range pod.Annotations {
				if strings.Contains(v, uid) {
					appNames = append(appNames, app)
				}
			}
		}
		mount.appPods = appNames
		mount.csiNode = ma.csiNodes[pod.Spec.NodeName]
		mount.status = getPodStatus(pod)
		ma.mounts = append(ma.mounts, mount)
	}

	if len(ma.mounts) == 0 {
		fmt.Printf("No mount pod found in %s namespace.", "kube-system")
		return nil
	}

	out, err := ma.printMountPods()
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	return nil
}

func (ma *mountAnalyzer) printMountPods() (string, error) {
	return tabbedString(func(out io.Writer) error {
		w := kdescribe.NewPrefixWriter(out)
		w.Write(kdescribe.LEVEL_0, "Name\tNamespace\tApp Pods\tStatus\tCSI Node\tNode\n")
		for _, pod := range ma.mounts {
			for i, app := range pod.appPods {
				name, ns, status, csiNode, node := "", "", "", "", ""
				appShow := app
				if i < len(pod.appPods)-1 {
					appShow = app + ","
				}
				if i == 0 {
					name, ns, status, csiNode, node = ifNil(pod.name), ifNil(pod.namespace), ifNil(pod.status), ifNil(pod.csiNode), ifNil(pod.node)
				}
				w.Write(kdescribe.LEVEL_0, "%s\t%s\t%s\t%s\t%s\t%s\n", name, ns, appShow, status, csiNode, node)
			}
			if len(pod.appPods) == 0 {
				w.Write(kdescribe.LEVEL_0, "%s\t%s\t%s\t%s\t%s\t%s\n", ifNil(pod.name), ifNil(pod.namespace), "<none>", ifNil(pod.status), ifNil(pod.csiNode), ifNil(pod.node))
			}
		}
		return nil
	})
}

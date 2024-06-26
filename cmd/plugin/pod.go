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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kdescribe "k8s.io/kubectl/pkg/describe"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

var podCmd = &cobra.Command{
	Use:     "pod",
	Aliases: []string{"po"},
	Short:   "Show pods using juicefs pvc",
	Run: func(cmd *cobra.Command, args []string) {
		ns, _ := rootCmd.Flags().GetString("namespace")
		if ns == "" {
			ns = "default"
		}
		aa, err := newAppAnalyzer(ns)
		cobra.CheckErr(err)
		cobra.CheckErr(aa.jfsPod(cmd, args))
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
	pvcs      map[string]corev1.PersistentVolumeClaim
	pvs       map[string]corev1.PersistentVolume

	apps []appPod
}

func newAppAnalyzer(ns string) (aa *appAnalyzer, err error) {
	clientSet, err := ClientSet(KubernetesConfigFlags)
	if err != nil {
		return nil, err
	}
	aa = &appAnalyzer{
		clientSet: clientSet,
		ns:        ns,
		pods:      make([]corev1.Pod, 0),
		mountPods: make([]corev1.Pod, 0),
		pvcs:      map[string]corev1.PersistentVolumeClaim{},
		pvs:       map[string]corev1.PersistentVolume{},
		apps:      make([]appPod, 0),
	}
	var (
		pvcList = make([]corev1.PersistentVolumeClaim, 0)
		pvList  = make([]corev1.PersistentVolume, 0)
	)
	if aa.pods, err = GetPodList(clientSet, ns); err != nil {
		return
	}
	if aa.mountPods, err = GetMountPodList(clientSet, ""); err != nil {
		return
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
	status    string
	createAt  metav1.Time
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
			status:    getPodStatus(pod),
			createAt:  pod.CreationTimestamp,
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
					if pv, ok := aa.pvs[pvc.Spec.VolumeName]; ok && pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
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
		w.Write(kdescribe.LEVEL_0, "NAME\tNAMESPACE\tMOUNT PODS\tSTATUS\tAGE\n")
		for _, pod := range aa.apps {
			for i, mount := range pod.mountPods {
				name, namespace, status, age := "", "", "", ""
				mountShow := mount
				if i < len(pod.mountPods)-1 {
					mountShow = mount + ","
				}
				if i == 0 {
					name, namespace, status, age = ifNil(pod.name), ifNil(pod.namespace), ifNil(pod.status), translateTimestampSince(pod.createAt)
				}
				w.Write(kdescribe.LEVEL_0, "%s\t%s\t%s\t%s\t%s\n", name, namespace, mountShow, status, age)
			}
			if len(pod.mountPods) == 0 {
				w.Write(kdescribe.LEVEL_0, "%s\t%s\t%s\t%s\t%s\n", ifNil(pod.name), ifNil(pod.namespace), "<none>", ifNil(pod.status), translateTimestampSince(pod.createAt))
			}
		}
		return nil
	})
}

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
	"k8s.io/client-go/kubernetes"
	kdescribe "k8s.io/kubectl/pkg/describe"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
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
		pod, err = clientSet.CoreV1().Pods(ns).Get(context.Background(), resourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod == nil {
			return fmt.Errorf("pod %s not found", resourceName)
		}
		describe, err = debugPod(clientSet, pod)
		if err != nil {
			return err
		}
		out, err = describePod(describe)
	}

	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	return nil
}

type podDescribe struct {
	name         string
	namespace    string
	node         *resourceStatus
	csiNode      *resourceStatus
	pvcs         []pvcStatus
	mountPods    []resourceStatus
	failedReason string
}

func (d *podDescribe) failed(reason string) {
	if d.failedReason == "" {
		d.failedReason = reason
	}
}

type pvcStatus struct {
	name      string
	namespace string
	pv        string
	status    string
}

type resourceStatus struct {
	name      string
	namespace string
	status    string
}

func debugPod(clientSet *kubernetes.Clientset, pod *corev1.Pod) (describe *podDescribe, err error) {
	if pod == nil {
		return nil, err
	}
	describe = &podDescribe{}
	describe.name = pod.Name
	describe.namespace = pod.Namespace

	var (
		node          *corev1.Node
		csiNode       *corev1.Pod
		mountPodsList *corev1.PodList
	)

	reason := ""
	// 1. PVC pending
	for _, volume := range pod.Spec.Volumes {
		var (
			pvc *corev1.PersistentVolumeClaim
			pv  *corev1.PersistentVolume
		)
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		pvc, err = clientSet.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(context.Background(), volume.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				return
			}
			reason = fmt.Sprintf("PVC [%s] not found.", volume.PersistentVolumeClaim.ClaimName)
			describe.failed(reason)
		}
		if pvc.Status.Phase != corev1.ClaimBound {
			reason = fmt.Sprintf("PVC [%s] is not bound.", pvc.Name)
			describe.failed(reason)
		} else {
			pv, err = clientSet.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			if err != nil {
				return
			}
			if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
				describe.pvcs = append(describe.pvcs, pvcStatus{
					name:      pvc.Name,
					namespace: pvc.Namespace,
					pv:        pv.Name,
					status:    string(pvc.Status.Phase),
				})
			}
		}
	}

	// 2. not scheduled
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status != corev1.ConditionTrue {
			reason = "Pod is not scheduled."
			describe.failed(reason)
		}
	}

	// 3. node not ready
	if pod.Spec.NodeName != "" {
		node, err = clientSet.CoreV1().Nodes().Get(context.Background(), pod.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			return
		}
		describe.node = &resourceStatus{
			name:   node.Name,
			status: "ready",
		}

		for _, condition := range node.Status.Conditions {
			if condition.Status == corev1.ConditionTrue {
				describe.node.status = string(condition.Type)
			}
			if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
				reason = fmt.Sprintf("Node [%s] is not ready", node.Name)
				describe.failed(reason)
			}
		}
	}

	// sidecar mode
	if pod.Labels != nil && pod.Labels["done.sidecar.juicefs.com/inject"] == "true" {
		reason = ""
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			if !containerStatus.Ready {
				reason = fmt.Sprintf("Init container [%s] is not ready", containerStatus.Name)
				describe.failed(reason)
				return
			}
		}
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				reason = fmt.Sprintf("Container [%s] is not ready", containerStatus.Name)
				describe.failed(reason)
				return
			}
		}
	} else {
		// mount pod mode
		// 4. check csi node
		csiNode, err = GetCSINode(clientSet, pod.Spec.NodeName)
		if err != nil {
			return
		}
		if csiNode == nil {
			describe.failed(fmt.Sprintf("CSI node not found on node [%s], please check if there are taints on node.", pod.Spec.NodeName))
		} else {
			describe.csiNode = &resourceStatus{
				name:      csiNode.Name,
				namespace: csiNode.Namespace,
				status:    string(csiNode.Status.Phase),
			}
			if !isPodReady(csiNode) {
				describe.failed(fmt.Sprintf("CSI node [%s] is not ready.", csiNode.Name))
			}
		}

		// 5. check mount pod
		mountPodsList, err = GetMountPodOnNode(clientSet, pod.Spec.NodeName)
		if err != nil {
			return
		}
		for _, mount := range mountPodsList.Items {
			for _, value := range mount.Annotations {
				if strings.Contains(value, string(pod.UID)) {
					describe.mountPods = append(describe.mountPods, resourceStatus{
						name:      mount.Name,
						namespace: mount.Namespace,
						status:    string(mount.Status.Phase),
					})
					if !isPodReady(&mount) {
						reason = fmt.Sprintf("Mount pod [%s] is not ready, please check its log.", mount.Name)
						describe.failed(reason)
					}
				}
			}
		}
		if len(mountPodsList.Items) == 0 {
			describe.failed("Mount pod not found, please check csi node's log for detail.")
		}
	}

	return
}

func describePod(describe *podDescribe) (string, error) {
	if describe == nil {
		return "", nil
	}
	return tabbedString(func(out io.Writer) error {
		w := kdescribe.NewPrefixWriter(out)
		w.Write(kdescribe.LEVEL_0, "Name:\t%s\n", describe.name)
		w.Write(kdescribe.LEVEL_0, "Namespace:\t%s\n", describe.namespace)

		w.Write(kdescribe.LEVEL_0, "Node: \n")
		if describe.node != nil {
			w.Write(kdescribe.LEVEL_1, "Name:\t%s\n", describe.node.name)
			w.Write(kdescribe.LEVEL_1, "Status:\t%s\n", describe.node.status)
		}

		w.Write(kdescribe.LEVEL_0, "CSI Node: \n")
		if describe.csiNode != nil {
			w.Write(kdescribe.LEVEL_1, "Name:\t%s\n", describe.csiNode.name)
			w.Write(kdescribe.LEVEL_1, "Namespace:\t%s\n", describe.csiNode.namespace)
			w.Write(kdescribe.LEVEL_1, "Status:\t%s\n", describe.node.status)
		}

		w.Write(kdescribe.LEVEL_0, "PVCs: \n")
		if len(describe.pvcs) > 0 {
			w.Write(kdescribe.LEVEL_1, "Name\tStatus\tPersistentVolume\n")
			w.Write(kdescribe.LEVEL_1, "----\t------\t----------------\n")
			for _, pvc := range describe.pvcs {
				w.Write(kdescribe.LEVEL_1, "%s\t%s\t%s\n", pvc.name, pvc.status, pvc.pv)
			}
		}

		w.Write(kdescribe.LEVEL_0, "Mount Pods: \n")
		if len(describe.mountPods) > 0 {
			w.Write(kdescribe.LEVEL_1, "Name\tNamespace\tStatus\n")
			w.Write(kdescribe.LEVEL_1, "----\t---------\t------\n")
			for _, pod := range describe.mountPods {
				w.Write(kdescribe.LEVEL_1, "%s\t%s\t%s\n", pod.name, pod.namespace, pod.status)
			}
		}
		if describe.failedReason != "" {
			w.Write(kdescribe.LEVEL_0, "Failed Reason:\n")
			w.Write(kdescribe.LEVEL_1, "%s\n", describe.failedReason)
		}
		return nil
	})
}

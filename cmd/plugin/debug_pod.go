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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kdescribe "k8s.io/kubectl/pkg/describe"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func newPodDescribe(clientSet *kubernetes.Clientset, pod *corev1.Pod) (describe *podDescribe, err error) {
	if pod == nil {
		return
	}
	describe = &podDescribe{}
	describe.name = pod.Name
	describe.namespace = pod.Namespace
	describe.status = getPodStatus(*pod)
	describe.pod = pod

	var (
		node          *corev1.Node
		csiNode       *corev1.Pod
		mountPodsList []corev1.Pod
	)

	for _, volume := range pod.Spec.Volumes {
		var (
			pvc *corev1.PersistentVolumeClaim
			pv  *corev1.PersistentVolume
		)
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		pvc, err = clientSet.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(context.Background(), volume.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return
		}
		if pvc.Status.Phase == corev1.ClaimBound {
			pv, err = clientSet.CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
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

	if pod.Spec.NodeName != "" {
		node, err = clientSet.CoreV1().Nodes().Get(context.Background(), pod.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			return
		}
		describe.node = &resourceStatus{
			name:   node.Name,
			status: "Ready",
		}

		for _, condition := range node.Status.Conditions {
			if condition.Status == corev1.ConditionTrue {
				describe.node.status = string(condition.Type)
			}
		}
	}

	// sidecar mode do not need
	if pod.Labels == nil || pod.Labels["done.sidecar.juicefs.com/inject"] != "true" {
		// mount pod mode
		csiNode, err = GetCSINode(clientSet, pod.Spec.NodeName)
		if err != nil {
			return
		}
		if csiNode != nil {
			describe.csiNodePod = csiNode
			describe.csiNode = &resourceStatus{
				name:      csiNode.Name,
				namespace: csiNode.Namespace,
				status:    string(csiNode.Status.Phase),
			}
		}

		mountPodsList, err = GetMountPodOnNode(clientSet, pod.Spec.NodeName)
		if err != nil {
			return
		}
		describe.mountPodList = make([]corev1.Pod, 0)
		for _, mount := range mountPodsList {
			for _, value := range mount.Annotations {
				if strings.Contains(value, string(pod.UID)) {
					describe.mountPodList = append(describe.mountPodList, mount)
					describe.mountPods = append(describe.mountPods, resourceStatus{
						name:      mount.Name,
						namespace: mount.Namespace,
						status:    getPodStatus(mount),
					})
				}
			}
		}
	}
	return
}

type podDescribe struct {
	pod          *corev1.Pod
	csiNodePod   *corev1.Pod
	mountPodList []corev1.Pod

	name         string
	namespace    string
	status       string
	node         *resourceStatus
	csiNode      *resourceStatus
	pvcs         []pvcStatus
	mountPods    []resourceStatus
	failedReason string
}

func (p *podDescribe) failed(reason string) {
	if p.failedReason == "" {
		p.failedReason = reason
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

func (p *podDescribe) debug() *podDescribe {
	if p.pod.DeletionTimestamp != nil {
		return p.debugTerminatingPod()
	}
	return p.debugRunningPod()
}

func (p *podDescribe) debugRunningPod() *podDescribe {
	// 1. PVC pending
	for _, pvc := range p.pvcs {
		if pvc.status != string(corev1.ClaimBound) {
			reason := fmt.Sprintf("PVC [%s] is not bound.", pvc.name)
			p.failed(reason)
		}
	}

	// 2. not scheduled
	for _, condition := range p.pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status != corev1.ConditionTrue {
			reason := "Pod is not scheduled."
			p.failed(reason)
		}
	}

	// 3. node not ready
	if p.node != nil && p.node.status != string(corev1.NodeReady) {
		reason := fmt.Sprintf("Node [%s] is not ready", p.node.name)
		p.failed(reason)
	}

	// sidecar mode do not need
	if p.pod.Labels == nil || p.pod.Labels["done.sidecar.juicefs.com/inject"] != "true" {
		// 4. csi node not ready
		if p.csiNode == nil {
			p.failed(fmt.Sprintf("CSI node not found on node [%s], please check if there are taints on node.", p.node))
		}
		if p.csiNodePod != nil && !isPodReady(*p.csiNodePod) {
			p.failed(fmt.Sprintf("CSI node [%s] is not ready.", p.node))
		}

		// 5. mount pod not ready
		for _, m := range p.mountPodList {
			if !isPodReady(m) {
				reason := fmt.Sprintf("Mount pod [%s] is not ready, please check its log.", m.Name)
				p.failed(reason)
			}
		}
		if len(p.pvcs) != 0 && len(p.mountPods) == 0 {
			p.failed("Mount pod not found, please check csi node's log for detail.")
		}
	}

	// 6. container error
	p.failed(getContainerErrorMessage(*p.pod))

	return p
}

func (p *podDescribe) debugTerminatingPod() *podDescribe {
	// 1. node not ready
	if p.node != nil && p.node.status != string(corev1.NodeReady) {
		reason := fmt.Sprintf("Node [%s] is not ready", p.node.name)
		p.failed(reason)
	}

	// sidecar mode do not need
	if p.pod.Labels == nil || p.pod.Labels["done.sidecar.juicefs.com/inject"] != "true" {
		// 2. csi node not ready
		if p.csiNode == nil {
			p.failed(fmt.Sprintf("CSI node not found on node [%s], please check if there are taints on node.", p.node))
		}
		if !isPodReady(*p.csiNodePod) {
			p.failed(fmt.Sprintf("CSI node [%s] is not ready.", p.node))
		}

		// 3. mount pod not terminating or contain pod uid
		for _, m := range p.mountPodList {
			if m.DeletionTimestamp != nil {
				p.failed(fmt.Sprintf("mount pod [%s] is still terminating", m.Name))
			} else {
				for _, value := range m.Annotations {
					if strings.Contains(value, string(p.pod.UID)) {
						p.failed(fmt.Sprintf("mount pod [%s] still contain its uid in annotations", m.Name))
					}
				}
			}
		}
	}

	// 4. container error
	p.failed(getContainerErrorMessage(*p.pod))

	// 5. finalizer not delete
	if p.pod.Finalizers != nil {
		p.failed(fmt.Sprintf("pod still has finalizer: %v", p.pod.Finalizers))
	}
	return p
}

func (p *podDescribe) describePod() (string, error) {
	return tabbedString(func(out io.Writer) error {
		w := kdescribe.NewPrefixWriter(out)
		w.Write(kdescribe.LEVEL_0, "Name:\t%s\n", p.name)
		w.Write(kdescribe.LEVEL_0, "Namespace:\t%s\n", p.namespace)
		w.Write(kdescribe.LEVEL_0, "Status:\t%s\n", p.status)

		w.Write(kdescribe.LEVEL_0, "Node: \n")
		if p.node != nil {
			w.Write(kdescribe.LEVEL_1, "Name:\t%s\n", p.node.name)
			w.Write(kdescribe.LEVEL_1, "Status:\t%s\n", p.node.status)
		}

		w.Write(kdescribe.LEVEL_0, "CSI Node: \n")
		if p.csiNode != nil {
			w.Write(kdescribe.LEVEL_1, "Name:\t%s\n", p.csiNode.name)
			w.Write(kdescribe.LEVEL_1, "Namespace:\t%s\n", p.csiNode.namespace)
			w.Write(kdescribe.LEVEL_1, "Status:\t%s\n", p.node.status)
		}

		w.Write(kdescribe.LEVEL_0, "PVCs: \n")
		if len(p.pvcs) > 0 {
			w.Write(kdescribe.LEVEL_1, "Name\tStatus\tPersistentVolume\n")
			w.Write(kdescribe.LEVEL_1, "----\t------\t----------------\n")
			for _, pvc := range p.pvcs {
				w.Write(kdescribe.LEVEL_1, "%s\t%s\t%s\n", pvc.name, pvc.status, pvc.pv)
			}
		}

		w.Write(kdescribe.LEVEL_0, "Mount Pods: \n")
		if len(p.mountPods) > 0 {
			w.Write(kdescribe.LEVEL_1, "Name\tNamespace\tStatus\n")
			w.Write(kdescribe.LEVEL_1, "----\t---------\t------\n")
			for _, pod := range p.mountPods {
				w.Write(kdescribe.LEVEL_1, "%s\t%s\t%s\n", pod.name, pod.namespace, pod.status)
			}
		}
		if p.failedReason != "" {
			w.Write(kdescribe.LEVEL_0, "Failed Reason:\n")
			w.Write(kdescribe.LEVEL_1, "%s\n", p.failedReason)
		}
		return nil
	})
}

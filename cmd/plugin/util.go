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
	"bytes"
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func ClientSet(configFlags *genericclioptions.ConfigFlags) *kubernetes.Clientset {
	restConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		panic("kube restConfig load error")
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic("gen kube restConfig error")
	}
	return clientSet
}

func GetMountPodList(clientSet *kubernetes.Clientset) (*corev1.PodList, error) {
	mountLabelMap, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{config.PodTypeKey: config.PodTypeValue},
	})
	mountList, err := clientSet.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{LabelSelector: mountLabelMap.String()})
	if err != nil {
		fmt.Printf("list mount pods error: %s", err.Error())
		return nil, err
	}
	return mountList, nil
}

func GetMountPodOnNode(clientSet *kubernetes.Clientset, nodeName string) (*corev1.PodList, error) {
	fieldSelector := fields.Set{"spec.nodeName": nodeName}
	mountLabelMap, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{config.PodTypeKey: config.PodTypeValue},
	})
	mountList, err := clientSet.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
		LabelSelector: mountLabelMap.String(),
		FieldSelector: fieldSelector.String(),
	})
	if err != nil {
		fmt.Printf("list mount pods error: %s", err.Error())
		return nil, err
	}
	return mountList, nil
}

func GetPodList(clientSet *kubernetes.Clientset, ns string) (*corev1.PodList, error) {
	podList, err := clientSet.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("list pods error: %s", err.Error())
		return nil, err
	}
	return podList, nil
}

func GetAppPodList(clientSet *kubernetes.Clientset, ns string) (*corev1.PodList, error) {
	labelMap, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      config.UniqueId,
			Operator: metav1.LabelSelectorOpExists,
		}},
	})
	podList, err := clientSet.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{LabelSelector: labelMap.String()})
	if err != nil {
		fmt.Printf("list pods error: %s", err.Error())
		return nil, err
	}
	return podList, nil
}

func GetCSINodeList(clientSet *kubernetes.Clientset) (*corev1.PodList, error) {
	nodeLabelMap, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{config.PodTypeKey: "juicefs-csi-driver", "app": "juicefs-csi-node"},
	})
	csiNodeList, err := clientSet.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{LabelSelector: nodeLabelMap.String()})
	if err != nil {
		fmt.Printf("list csi node pods error: %s", err.Error())
		return nil, err
	}
	return csiNodeList, nil
}

func GetCSINode(clientSet *kubernetes.Clientset, nodeName string) (*corev1.Pod, error) {
	fieldSelector := fields.Set{"spec.nodeName": nodeName}
	nodeLabelMap, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{config.PodTypeKey: "juicefs-csi-driver", "app": "juicefs-csi-node"},
	})
	csiNodeList, err := clientSet.CoreV1().Pods("kube-system").List(context.Background(),
		metav1.ListOptions{
			LabelSelector: nodeLabelMap.String(),
			FieldSelector: fieldSelector.String(),
		})
	if err != nil {
		fmt.Printf("list csi node pods error: %s", err.Error())
		return nil, err
	}
	if csiNodeList == nil || len(csiNodeList.Items) == 0 {
		return nil, nil
	}
	return &csiNodeList.Items[0], nil
}

func GetNamespaceList(clientSet *kubernetes.Clientset) (*corev1.NamespaceList, error) {
	namespaces, err := clientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("list namespaces error: %s", err.Error())
		return nil, err
	}
	return namespaces, nil
}

func tabbedString(f func(io.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 2, ' ', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	out.Flush()
	return buf.String(), nil
}

func isPodReady(pod *corev1.Pod) bool {
	conditionsTrue := 0
	for _, cond := range pod.Status.Conditions {
		if cond.Status == corev1.ConditionTrue && (cond.Type == corev1.ContainersReady || cond.Type == corev1.PodReady) {
			conditionsTrue++
		}
	}
	return conditionsTrue == 2
}

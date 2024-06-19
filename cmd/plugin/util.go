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

	"github.com/liushuochen/gotable"
	"github.com/liushuochen/gotable/table"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func isSysPod(pod *corev1.Pod) bool {
	if pod.Labels != nil {
		return pod.Labels["app.kubernetes.io/name"] == "juicefs-mount" || pod.Labels["app.kubernetes.io/name"] == "juicefs-csi-driver"
	}
	return false
}

func isCsiNode(pod *corev1.Pod) bool {
	if pod.Labels != nil {
		return pod.Labels["app.kubernetes.io/name"] == "juicefs-csi-driver" && pod.Labels["app"] == "juicefs-csi-node"
	}
	return false
}

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

func GenTable(title []string, mapList []map[string]string) *table.Table {
	t, err := gotable.Create(title...)
	if err != nil {
		fmt.Printf("create table error: %s", err.Error())
		return nil
	}
	t.AddRows(mapList)
	return t
}

func GetMountPods(clientSet *kubernetes.Clientset) (*corev1.PodList, error) {
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

func GetAppPods(clientSet *kubernetes.Clientset, ns string) (*corev1.PodList, error) {
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

func GetCSINode(clientSet *kubernetes.Clientset) (*corev1.PodList, error) {
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

func GetNamespaces(clientSet *kubernetes.Clientset) (*corev1.NamespaceList, error) {
	namespaces, err := clientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("list namespaces error: %s", err.Error())
		return nil, err
	}
	return namespaces, nil
}

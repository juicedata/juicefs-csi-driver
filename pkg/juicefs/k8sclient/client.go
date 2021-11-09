/*
Copyright 2021 Juicedata Inc

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

package k8sclient

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

type K8sClient interface {
	CreatePod(pod *corev1.Pod) (*corev1.Pod, error)
	GetPod(podName, namespace string) (*corev1.Pod, error)
	PatchPod(pod *corev1.Pod, data []byte) error
	UpdatePod(pod *corev1.Pod) error
	DeletePod(pod *corev1.Pod) error
}

type k8sClient struct {
	*kubernetes.Clientset
}

func NewClient() (K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &k8sClient{client}, nil
}

func (k *k8sClient) CreatePod(pod *corev1.Pod) (*corev1.Pod, error) {
	klog.V(5).Infof("Create pod %s", pod.Name)
	mntPod, err := k.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		klog.V(5).Infof("Can't create pod %s: %v", pod.Name, err)
		return nil, err
	}
	return mntPod, nil
}

func (k *k8sClient) GetPod(podName, namespace string) (*corev1.Pod, error) {
	klog.V(6).Infof("Get pod %s", podName)
	mntPod, err := k.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		klog.V(5).Infof("Can't get pod %s namespace %s: %v", podName, namespace, err)
		return nil, err
	}
	return mntPod, nil
}

func (k *k8sClient) PatchPod(pod *corev1.Pod, data []byte) error {
	klog.V(5).Infof("Patch pod %v", pod.Name)
	_, err := k.CoreV1().Pods(pod.Namespace).Patch(context.TODO(),
		pod.Name, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	return err
}

func (k *k8sClient) UpdatePod(pod *corev1.Pod) error {
	klog.V(5).Infof("Update pod %v", pod.Name)
	_, err := k.CoreV1().Pods(pod.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
	return err
}

func (k *k8sClient) DeletePod(pod *corev1.Pod) error {
	klog.V(5).Infof("Delete pod %v", pod.Name)
	return k.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
}

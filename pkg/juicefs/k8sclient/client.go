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
	"bytes"
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"time"
)

const (
	timeout = 10 * time.Second
)

type K8sClient struct {
	kubernetes.Interface
}

func NewClient() (*K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, status.Error(codes.NotFound, "Can't get kube InClusterConfig")
	}
	config.Timeout = timeout
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &K8sClient{client}, nil
}

func (k *K8sClient) CreatePod(pod *corev1.Pod) (*corev1.Pod, error) {
	if pod == nil {
		klog.V(5).Info("Create pod: pod is nil")
		return nil, nil
	}
	klog.V(6).Infof("Create pod %s", pod.Name)
	mntPod, err := k.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		klog.V(5).Infof("Can't create pod %s: %v", pod.Name, err)
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetPod(podName, namespace string) (*corev1.Pod, error) {
	klog.V(6).Infof("Get pod %s", podName)
	mntPod, err := k.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		klog.V(6).Infof("Can't get pod %s namespace %s: %v", podName, namespace, err)
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetPodLog(podName, namespace, containerName string) (string, error) {
	klog.V(6).Infof("Get pod %s log", podName)
	tailLines := int64(5)
	req := k.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
	})
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	str := buf.String()
	return str, nil
}

func (k *K8sClient) PatchPod(pod *corev1.Pod, data []byte) error {
	if pod == nil {
		klog.V(5).Info("Patch pod: pod is nil")
		return nil
	}
	klog.V(6).Infof("Patch pod %v", pod.Name)
	_, err := k.CoreV1().Pods(pod.Namespace).Patch(context.TODO(),
		pod.Name, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	return err
}

func (k *K8sClient) UpdatePod(pod *corev1.Pod) error {
	if pod == nil {
		klog.V(5).Info("Update pod: pod is nil")
		return nil
	}
	klog.V(6).Infof("Update pod %v", pod.Name)
	_, err := k.CoreV1().Pods(pod.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeletePod(pod *corev1.Pod) error {
	if pod == nil {
		klog.V(5).Info("Delete pod: pod is nil")
		return nil
	}
	klog.V(6).Infof("Delete pod %v", pod.Name)
	return k.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
}

func (k *K8sClient) GetSecret(secretName, namespace string) (*corev1.Secret, error) {
	klog.V(6).Infof("Get secret %s", secretName)
	secret, err := k.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		klog.V(6).Infof("Can't get secret %s namespace %s: %v", secretName, namespace, err)
		return nil, err
	}
	return secret, nil
}

func (k *K8sClient) CreateSecret(secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil {
		klog.V(5).Info("Create secret: secret is nil")
		return nil, nil
	}
	klog.V(6).Infof("Create secret %s", secret.Name)
	secret, err := k.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		klog.V(5).Infof("Can't create secret %s: %v", secret.Name, err)
		return nil, err
	}
	return secret, nil
}

func (k *K8sClient) UpdateSecret(secret *corev1.Secret) error {
	if secret == nil {
		klog.V(5).Info("Update secret: secret is nil")
		return nil
	}
	klog.V(6).Infof("Update secret %v", secret.Name)
	_, err := k.CoreV1().Secrets(secret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeleteSecret(secretName string, namespace string) error {
	klog.V(6).Infof("Delete secret %s", secretName)
	return k.CoreV1().Secrets(namespace).Delete(context.TODO(), secretName, metav1.DeleteOptions{})
}

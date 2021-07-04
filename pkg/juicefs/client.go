/*

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

package juicefs

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

func NewClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func GetOrCreatePod(k8sClient *kubernetes.Clientset, pod *corev1.Pod) (*corev1.Pod, error) {
	klog.V(5).Infof("Get pod %s", pod.Name)
	mntPod, err := k8sClient.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		// if not exist, create pod
		klog.V(5).Infof("Pod %s does not exist, create it.", pod.Name)
		mntPod, err = k8sClient.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		if err != nil {
			klog.V(5).Infof("Can't create pod %s: %v", pod.Name, err)
			return nil, err
		}
		return mntPod, nil
	} else if err != nil {
		klog.V(5).Infof("Can't get pod %s: %v", pod.Name, err)
		return nil, err
	}
	return mntPod, nil
}

func CreatePod(k8sClient *kubernetes.Clientset, pod *corev1.Pod) (*corev1.Pod, error) {
	klog.V(5).Infof("Create pod %s", pod.Name)
	mntPod, err := k8sClient.CoreV1().Pods(pod.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		klog.V(5).Infof("Can't create pod %s: %v", pod.Name, err)
		return nil, err
	}
	return mntPod, nil
}

func GetPod(k8sClient *kubernetes.Clientset, podName, namespace string) (*corev1.Pod, error) {
	klog.V(5).Infof("Get pod %s", podName)
	mntPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		klog.V(5).Infof("Can't get pod %s namespace %s: %v", podName, namespace, err)
		return nil, err
	}
	return mntPod, nil
}

func DeletePod(k8sClient *kubernetes.Clientset, pod *corev1.Pod) error {
	klog.V(5).Infof("Delete pod %v", pod.Name)
	return k8sClient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
}

func GetOrCreateConfigMap(k8sClient *kubernetes.Clientset, cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	klog.V(5).Infof("Get configMap %s", cm.Name)
	exit, err := k8sClient.CoreV1().ConfigMaps(cm.Namespace).Get(context.TODO(), cm.Name, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		// if not exist, create cm
		klog.V(5).Infof("ConfigMap %s does not exist, create it.", cm.Name)
		created, e := k8sClient.CoreV1().ConfigMaps(cm.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
		if e != nil {
			klog.V(5).Infof("Can't create configMap %s: %v", cm.Name, err)
			return nil, e
		}
		return created, nil
	} else if err != nil {
		klog.V(5).Infof("Can't get configMap %s: %v", cm.Name, err)
		return nil, err
	}
	return exit, nil
}

func GetConfigMap(k8sClient *kubernetes.Clientset, cmName, namespace string) (*corev1.ConfigMap, error) {
	klog.V(5).Infof("Get configMap %s", cmName)
	cm, err := k8sClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		klog.V(5).Infof("Can't get configMap %s: %v", cmName, err)
		return nil, err
	}
	return cm, nil
}

func UpdateConfigMap(k8sClient *kubernetes.Clientset, cm *corev1.ConfigMap) error {
	klog.V(5).Infof("Update configMap %v", cm.Name)
	_, err := k8sClient.CoreV1().ConfigMaps(cm.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	return err
}

func DeleteConfigMap(k8sClient *kubernetes.Clientset, cm *corev1.ConfigMap) error {
	klog.V(5).Infof("Delete configMap %v", cm.Name)
	return k8sClient.CoreV1().ConfigMaps(cm.Namespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{})
}

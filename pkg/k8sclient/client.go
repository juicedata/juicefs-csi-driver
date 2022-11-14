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
	"io"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

const (
	timeout = 10 * time.Second
)

type PatchListValue struct {
	Op    string   `json:"op"`
	Path  string   `json:"path"`
	Value []string `json:"value"`
}

type PatchMapValue struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

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

func (k *K8sClient) CreatePod(ctx context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
	if pod == nil {
		klog.V(5).Info("Create pod: pod is nil")
		return nil, nil
	}
	klog.V(6).Infof("Create pod %s", pod.Name)
	mntPod, err := k.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		klog.V(5).Infof("Can't create pod %s: %v", pod.Name, err)
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetPod(ctx context.Context, podName, namespace string) (*corev1.Pod, error) {
	klog.V(6).Infof("Get pod %s", podName)
	mntPod, err := k.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		klog.V(6).Infof("Can't get pod %s namespace %s: %v", podName, namespace, err)
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) ListPod(ctx context.Context, namespace string, labelSelector *metav1.LabelSelector, filedSelector *fields.Set) ([]corev1.Pod, error) {
	klog.V(6).Infof("List pod by labelSelector %v, fieldSelector %v", labelSelector, filedSelector)
	listOptions := metav1.ListOptions{}
	if labelSelector != nil {
		labelMap, err := metav1.LabelSelectorAsMap(labelSelector)
		if err != nil {
			return nil, err
		}
		listOptions.LabelSelector = labels.SelectorFromSet(labelMap).String()
	}
	if filedSelector != nil {
		listOptions.FieldSelector = fields.SelectorFromSet(*filedSelector).String()
	}

	podList, err := k.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		klog.V(6).Infof("Can't list pod in namespace %s by labelSelector %v: %v", namespace, labelSelector, err)
		return nil, err
	}
	return podList.Items, nil
}

func (k *K8sClient) GetPodLog(ctx context.Context, podName, namespace, containerName string) (string, error) {
	klog.V(6).Infof("Get pod %s log", podName)
	tailLines := int64(20)
	req := k.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
	})
	podLogs, err := req.Stream(ctx)
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

func (k *K8sClient) PatchPod(ctx context.Context, pod *corev1.Pod, data []byte) error {
	if pod == nil {
		klog.V(5).Info("Patch pod: pod is nil")
		return nil
	}
	klog.V(6).Infof("Patch pod %v", pod.Name)
	_, err := k.CoreV1().Pods(pod.Namespace).Patch(ctx,
		pod.Name, types.JSONPatchType, data, metav1.PatchOptions{})
	return err
}

func (k *K8sClient) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		klog.V(5).Info("Update pod: pod is nil")
		return nil
	}
	klog.V(6).Infof("Update pod %v", pod.Name)
	_, err := k.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		klog.V(5).Info("Delete pod: pod is nil")
		return nil
	}
	klog.V(6).Infof("Delete pod %v", pod.Name)
	return k.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
}

func (k *K8sClient) GetSecret(ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
	klog.V(6).Infof("Get secret %s", secretName)
	secret, err := k.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		klog.V(6).Infof("Can't get secret %s namespace %s: %v", secretName, namespace, err)
		return nil, err
	}
	return secret, nil
}

func (k *K8sClient) CreateSecret(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil {
		klog.V(5).Info("Create secret: secret is nil")
		return nil, nil
	}
	klog.V(6).Infof("Create secret %s", secret.Name)
	secret, err := k.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		klog.V(5).Infof("Can't create secret %s: %v", secret.Name, err)
		return nil, err
	}
	return secret, nil
}

func (k *K8sClient) UpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	if secret == nil {
		klog.V(5).Info("Update secret: secret is nil")
		return nil
	}
	klog.V(6).Infof("Update secret %v", secret.Name)
	_, err := k.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeleteSecret(ctx context.Context, secretName string, namespace string) error {
	klog.V(6).Infof("Delete secret %s", secretName)
	return k.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
}

func (k *K8sClient) GetJob(ctx context.Context, jobName, namespace string) (*batchv1.Job, error) {
	klog.V(6).Infof("Get job %s", jobName)
	job, err := k.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		klog.V(6).Infof("Can't get job %s namespace %s: %v", jobName, namespace, err)
		return nil, err
	}
	return job, nil
}

func (k *K8sClient) CreateJob(ctx context.Context, job *batchv1.Job) (*batchv1.Job, error) {
	if job == nil {
		klog.V(5).Info("Create job: job is nil")
		return nil, nil
	}
	klog.V(6).Infof("Create job %s", job.Name)
	created, err := k.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		klog.V(5).Infof("Can't create job %s: %v", job.Name, err)
		return nil, err
	}
	return created, nil
}

func (k *K8sClient) UpdateJob(ctx context.Context, job *batchv1.Job) error {
	if job == nil {
		klog.V(5).Info("Update job: job is nil")
		return nil
	}
	klog.V(6).Infof("Update job %v", job.Name)
	_, err := k.BatchV1().Jobs(job.Namespace).Update(ctx, job, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeleteJob(ctx context.Context, jobName string, namespace string) error {
	klog.V(6).Infof("Delete job %s", jobName)
	policy := metav1.DeletePropagationBackground
	return k.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
}

func (k *K8sClient) GetPersistentVolume(ctx context.Context, pvName string) (*corev1.PersistentVolume, error) {
	klog.V(6).Infof("Get pv %s", pvName)
	mntPod, err := k.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		klog.V(6).Infof("Can't get pv %s : %v", pvName, err)
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetAdmissionWebhookConfig(ctx context.Context, name string) (*admissionv1.MutatingWebhookConfiguration, error) {
	klog.V(6).Infof("Get admission webhook %s", name)
	wh, err := k.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		klog.V(6).Infof("Can't get admission webhook %s: %v", name, err)
		return nil, err
	}
	return wh, nil
}

func (k *K8sClient) PatchAdmissionWebhookConfig(ctx context.Context, whName string, data []byte) error {
	klog.V(6).Infof("Update admission webhook %v", whName)
	_, err := k.AdmissionregistrationV1().MutatingWebhookConfigurations().Patch(ctx, whName, types.MergePatchType, data, metav1.PatchOptions{})
	return err
}

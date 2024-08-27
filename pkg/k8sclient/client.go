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
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

const (
	timeout = 10 * time.Second
)

var clientLog = klog.NewKlogr().WithName("client")

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

type PatchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type PatchDelValue struct {
	Op   string `json:"op"`
	Path string `json:"path"`
}

type K8sClient struct {
	enableAPIServerListCache bool
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

	if os.Getenv("KUBE_QPS") != "" {
		kubeQpsInt, err := strconv.Atoi(os.Getenv("KUBE_QPS"))
		if err != nil {
			return nil, err
		}
		config.QPS = float32(kubeQpsInt)
	}
	if os.Getenv("KUBE_BURST") != "" {
		kubeBurstInt, err := strconv.Atoi(os.Getenv("KUBE_BURST"))
		if err != nil {
			return nil, err
		}
		config.Burst = kubeBurstInt
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	var enableAPIServerListCache bool
	if os.Getenv("ENABLE_APISERVER_LIST_CACHE") == "true" {
		enableAPIServerListCache = true
	}
	return &K8sClient{enableAPIServerListCache, client}, nil
}

func (k *K8sClient) CreatePod(ctx context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
	if pod == nil {
		clientLog.Info("Create pod: pod is nil")
		return nil, nil
	}
	clientLog.V(1).Info("Create pod", "name", pod.Name)
	mntPod, err := k.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		clientLog.Info("Can't create pod", "name", pod.Name, "error", err)
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetPod(ctx context.Context, podName, namespace string) (*corev1.Pod, error) {
	clientLog.V(1).Info("Get pod", "name", podName)
	mntPod, err := k.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		clientLog.V(1).Info("Can't get pod", "name", podName, "namespace", namespace, "error", err)
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) ListPod(ctx context.Context, namespace string, labelSelector *metav1.LabelSelector, filedSelector *fields.Set) ([]corev1.Pod, error) {
	clientLog.V(1).Info("List pod", "labelSelector", labelSelector.String(), "fieldSelector", filedSelector)
	listOptions := metav1.ListOptions{}
	if k.enableAPIServerListCache {
		// set ResourceVersion="0" means the list response is returned from apiserver cache instead of etcd
		listOptions.ResourceVersion = "0"
	}
	if labelSelector != nil {
		labelMap, err := metav1.LabelSelectorAsSelector(labelSelector)
		if err != nil {
			return nil, err
		}
		listOptions.LabelSelector = labelMap.String()
	}
	if filedSelector != nil {
		listOptions.FieldSelector = fields.SelectorFromSet(*filedSelector).String()
	}

	podList, err := k.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		clientLog.V(1).Info("Can't list pod", "namespace", namespace, "labelSelector", labelSelector.String(), "error", err)
		return nil, err
	}
	return podList.Items, nil
}

func (k *K8sClient) ListNode(ctx context.Context, labelSelector *metav1.LabelSelector) ([]corev1.Node, error) {
	clientLog.V(1).Info("List node by labelSelector", "labelSelector", labelSelector.String())
	listOptions := metav1.ListOptions{}
	if labelSelector != nil {
		labelMap, err := metav1.LabelSelectorAsSelector(labelSelector)
		if err != nil {
			return nil, err
		}
		listOptions.LabelSelector = labelMap.String()
	}

	nodeList, err := k.CoreV1().Nodes().List(ctx, listOptions)
	if err != nil {
		clientLog.V(1).Info("Can't list node", "labelSelector", labelSelector.String(), "error", err)
		return nil, err
	}
	return nodeList.Items, nil
}

func (k *K8sClient) GetPodLog(ctx context.Context, podName, namespace, containerName string) (string, error) {
	clientLog.V(1).Info("Get pod log", "name", podName)
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

func (k *K8sClient) PatchPod(ctx context.Context, pod *corev1.Pod, data []byte, pt types.PatchType) error {
	if pod == nil {
		clientLog.Info("Patch pod: pod is nil")
		return nil
	}
	clientLog.V(1).Info("Patch pod", "name", pod.Name)
	_, err := k.CoreV1().Pods(pod.Namespace).Patch(ctx, pod.Name, pt, data, metav1.PatchOptions{})
	return err
}

func (k *K8sClient) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		clientLog.Info("Update pod: pod is nil")
		return nil
	}
	clientLog.V(1).Info("Update pod", "name", pod.Name)
	_, err := k.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		clientLog.Info("Delete pod: pod is nil")
		return nil
	}
	clientLog.V(1).Info("Delete pod", "name", pod.Name)
	return k.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
}

func (k *K8sClient) GetSecret(ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
	clientLog.V(1).Info("Get secret", "name", secretName)
	secret, err := k.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		clientLog.V(1).Info("Can't get secret", "name", secretName, "namespace", namespace, "error", err)
		return nil, err
	}
	return secret, nil
}

func (k *K8sClient) CreateSecret(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil {
		clientLog.Info("Create secret: secret is nil")
		return nil, nil
	}
	clientLog.V(1).Info("Create secret", "name", secret.Name)
	s, err := k.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		clientLog.Info("Can't create secret", "name", secret.Name, "error", err)
		return nil, err
	}
	return s, nil
}

func (k *K8sClient) UpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	if secret == nil {
		clientLog.Info("Update secret: secret is nil")
		return nil
	}
	clientLog.V(1).Info("Update secret", "name", secret.Name)
	_, err := k.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeleteSecret(ctx context.Context, secretName string, namespace string) error {
	clientLog.V(1).Info("Delete secret", "name", secretName)
	return k.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
}

func (k *K8sClient) PatchSecret(ctx context.Context, secret *corev1.Secret, data []byte, pt types.PatchType) error {
	if secret == nil {
		clientLog.Info("Patch secret: secret is nil")
		return nil
	}
	clientLog.V(1).Info("Patch secret", "name", secret.Name)
	_, err := k.CoreV1().Secrets(secret.Namespace).Patch(ctx, secret.Name, pt, data, metav1.PatchOptions{})
	return err
}

func (k *K8sClient) GetJob(ctx context.Context, jobName, namespace string) (*batchv1.Job, error) {
	clientLog.V(1).Info("Get job", "name", jobName)
	job, err := k.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		clientLog.V(1).Info("Can't get job", "name", jobName, "namespace", namespace, "error", err)
		return nil, err
	}
	return job, nil
}

func (k *K8sClient) CreateJob(ctx context.Context, job *batchv1.Job) (*batchv1.Job, error) {
	if job == nil {
		clientLog.Info("Create job: job is nil")
		return nil, nil
	}
	clientLog.V(1).Info("Create job", "name", job.Name)
	created, err := k.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		clientLog.Info("Can't create job", "name", job.Name, "error", err)
		return nil, err
	}
	return created, nil
}

func (k *K8sClient) UpdateJob(ctx context.Context, job *batchv1.Job) error {
	if job == nil {
		clientLog.Info("Update job: job is nil")
		return nil
	}
	clientLog.V(1).Info("Update job", "name", job.Name)
	_, err := k.BatchV1().Jobs(job.Namespace).Update(ctx, job, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeleteJob(ctx context.Context, jobName string, namespace string) error {
	clientLog.V(1).Info("Delete job", "name", jobName)
	policy := metav1.DeletePropagationBackground
	return k.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
}

func (k *K8sClient) GetPersistentVolume(ctx context.Context, pvName string) (*corev1.PersistentVolume, error) {
	clientLog.V(1).Info("Get pv", "name", pvName)
	pv, err := k.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		clientLog.V(1).Info("Can't get pv", "name", pvName, "error", err)
		return nil, err
	}
	return pv, nil
}

func (k *K8sClient) ListPersistentVolumes(ctx context.Context, labelSelector *metav1.LabelSelector, filedSelector *fields.Set) ([]corev1.PersistentVolume, error) {
	clientLog.V(1).Info("List pvs", "labelSelector", labelSelector.String(), "fieldSelector", filedSelector)
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
	pvList, err := k.CoreV1().PersistentVolumes().List(ctx, listOptions)
	if err != nil {
		clientLog.V(1).Info("Can't list pv", "error", err)
		return nil, err
	}
	return pvList.Items, nil
}

func (k *K8sClient) ListPersistentVolumesByVolumeHandle(ctx context.Context, volumeHandle string) ([]corev1.PersistentVolume, error) {
	pvs, err := k.ListPersistentVolumes(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	var result []corev1.PersistentVolume
	for _, pv := range pvs {
		pv := pv
		if pv.Spec.CSI != nil && pv.Spec.CSI.VolumeHandle == volumeHandle {
			result = append(result, pv)
		}
	}
	return result, nil
}

func (k *K8sClient) ListStorageClasses(ctx context.Context) ([]storagev1.StorageClass, error) {
	clientLog.V(1).Info("List storageclass")
	scList, err := k.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		clientLog.V(1).Info("Can't list pv", "error", err)
		return nil, err
	}
	return scList.Items, nil
}

func (k *K8sClient) GetPersistentVolumeClaim(ctx context.Context, pvcName, namespace string) (*corev1.PersistentVolumeClaim, error) {
	clientLog.V(1).Info("Get pvc", "name", pvcName, "namespace", namespace)
	mntPod, err := k.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		clientLog.V(1).Info("Can't get pvc", "name", pvcName, "namespace", namespace, "error", err)
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetReplicaSet(ctx context.Context, rsName, namespace string) (*appsv1.ReplicaSet, error) {
	clientLog.V(1).Info("Get replicaset", "name", rsName, "namespace", namespace)
	rs, err := k.AppsV1().ReplicaSets(namespace).Get(ctx, rsName, metav1.GetOptions{})
	if err != nil {
		clientLog.V(1).Info("Can't get replicaset", "name", rsName, "namespace", namespace, "error", err)
		return nil, err
	}
	return rs, nil
}

func (k *K8sClient) GetStatefulSet(ctx context.Context, stsName, namespace string) (*appsv1.StatefulSet, error) {
	clientLog.V(1).Info("Get statefulset", "name", stsName, "namespace", namespace)
	sts, err := k.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
	if err != nil {
		clientLog.V(1).Info("Can't get statefulset", "name", stsName, "namespace", namespace, "error", err)
		return nil, err
	}
	return sts, nil
}

func (k *K8sClient) GetStorageClass(ctx context.Context, scName string) (*storagev1.StorageClass, error) {
	clientLog.V(1).Info("Get sc", "name", scName)
	mntPod, err := k.StorageV1().StorageClasses().Get(ctx, scName, metav1.GetOptions{})
	if err != nil {
		clientLog.V(1).Info("Can't get sc", "name", scName, "error", err)
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetDaemonSet(ctx context.Context, dsName, namespace string) (*appsv1.DaemonSet, error) {
	clientLog.V(1).Info("Get ds", "name", dsName)
	ds, err := k.AppsV1().DaemonSets(namespace).Get(ctx, dsName, metav1.GetOptions{})
	if err != nil {
		clientLog.Info("Can't get DaemonSet", "name", dsName, "error", err)
		return nil, err
	}
	return ds, nil
}

func (k *K8sClient) ExecuteInContainer(podName, namespace, containerName string, cmd []string) (stdout string, stderr string, err error) {
	clientLog.V(1).Info("Execute command pod", "command", cmd, "container", containerName, "pod", podName, "namespace", namespace)
	const tty = false

	req := k.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Param("container", containerName)
	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       tty,
	}, scheme.ParameterCodec)

	var sout, serr bytes.Buffer
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", "", err
	}
	config.Timeout = timeout
	err = execute("POST", req.URL(), config, nil, &sout, &serr, tty)

	return strings.TrimSpace(sout.String()), strings.TrimSpace(serr.String()), err
}

func execute(method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool) error {
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
}

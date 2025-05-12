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
	"fmt"
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
)

const (
	timeout = 10 * time.Second
)

type PatchListValue struct {
	Op    string   `json:"op"`
	Path  string   `json:"path"`
	Value []string `json:"value"`
}

type PatchInterfaceValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
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
	kubeQPS := float32(20.0)
	kubeBurst := 30
	if os.Getenv("KUBE_QPS") != "" {
		kubeQpsInt, err := strconv.Atoi(os.Getenv("KUBE_QPS"))
		if err != nil {
			return nil, err
		}
		kubeQPS = float32(kubeQpsInt)
	}
	if os.Getenv("KUBE_BURST") != "" {
		kubeBurst, err = strconv.Atoi(os.Getenv("KUBE_BURST"))
		if err != nil {
			return nil, err
		}
	}
	config.QPS = kubeQPS
	config.Burst = kubeBurst
	return newClient(*config)
}

func NewClientWithConfig(config rest.Config) (*K8sClient, error) {
	return newClient(config)
}

func newClient(config rest.Config) (*K8sClient, error) {
	client, err := kubernetes.NewForConfig(&config)
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
		return nil, nil
	}
	mntPod, err := k.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetPod(ctx context.Context, podName, namespace string) (*corev1.Pod, error) {
	mntPod, err := k.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) ListPod(ctx context.Context, namespace string, labelSelector *metav1.LabelSelector, filedSelector *fields.Set) ([]corev1.Pod, error) {
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
		return nil, err
	}
	return podList.Items, nil
}

func (k *K8sClient) ListNode(ctx context.Context, labelSelector *metav1.LabelSelector) ([]corev1.Node, error) {
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
		return nil, err
	}
	return nodeList.Items, nil
}

func (k *K8sClient) GetPodLog(ctx context.Context, podName, namespace, containerName string) (string, error) {
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

func (k *K8sClient) PatchPod(ctx context.Context, podName, namespace string, data []byte, pt types.PatchType) error {
	_, err := k.CoreV1().Pods(namespace).Patch(ctx, podName, pt, data, metav1.PatchOptions{})
	return err
}

func (k *K8sClient) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}
	_, err := k.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}
	return k.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
}

func (k *K8sClient) GetSecret(ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
	secret, err := k.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (k *K8sClient) CreateSecret(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil {
		return nil, nil
	}
	s, err := k.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (k *K8sClient) UpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	if secret == nil {
		return nil
	}
	_, err := k.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeleteSecret(ctx context.Context, secretName string, namespace string) error {
	return k.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
}

func (k *K8sClient) PatchSecret(ctx context.Context, secret *corev1.Secret, data []byte, pt types.PatchType) error {
	if secret == nil {
		return nil
	}

	_, err := k.CoreV1().Secrets(secret.Namespace).Patch(ctx, secret.Name, pt, data, metav1.PatchOptions{})
	return err
}

func (k *K8sClient) GetJob(ctx context.Context, jobName, namespace string) (*batchv1.Job, error) {
	job, err := k.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (k *K8sClient) CreateJob(ctx context.Context, job *batchv1.Job) (*batchv1.Job, error) {
	if job == nil {
		return nil, nil
	}
	created, err := k.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (k *K8sClient) UpdateJob(ctx context.Context, job *batchv1.Job) error {
	if job == nil {
		return nil
	}
	_, err := k.BatchV1().Jobs(job.Namespace).Update(ctx, job, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) DeleteJob(ctx context.Context, jobName string, namespace string) error {
	policy := metav1.DeletePropagationBackground
	return k.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
}

func (k *K8sClient) GetPersistentVolume(ctx context.Context, pvName string) (*corev1.PersistentVolume, error) {
	pv, err := k.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pv, nil
}

func (k *K8sClient) ListPersistentVolumes(ctx context.Context, labelSelector *metav1.LabelSelector, filedSelector *fields.Set) ([]corev1.PersistentVolume, error) {
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
	scList, err := k.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return scList.Items, nil
}

func (k *K8sClient) GetPersistentVolumeClaim(ctx context.Context, pvcName, namespace string) (*corev1.PersistentVolumeClaim, error) {
	mntPod, err := k.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetReplicaSet(ctx context.Context, rsName, namespace string) (*appsv1.ReplicaSet, error) {
	rs, err := k.AppsV1().ReplicaSets(namespace).Get(ctx, rsName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (k *K8sClient) GetStatefulSet(ctx context.Context, stsName, namespace string) (*appsv1.StatefulSet, error) {
	sts, err := k.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return sts, nil
}

func (k *K8sClient) GetStorageClass(ctx context.Context, scName string) (*storagev1.StorageClass, error) {
	mntPod, err := k.StorageV1().StorageClasses().Get(ctx, scName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return mntPod, nil
}

func (k *K8sClient) GetDaemonSet(ctx context.Context, dsName, namespace string) (*appsv1.DaemonSet, error) {
	ds, err := k.AppsV1().DaemonSets(namespace).Get(ctx, dsName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return ds, nil
}

func (k *K8sClient) ExecuteInContainer(ctx context.Context, podName, namespace, containerName string, cmd []string) (stdout string, stderr string, err error) {
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
	err = execute(ctx, "POST", req.URL(), config, nil, &sout, &serr, tty)

	return strings.TrimSpace(sout.String()), strings.TrimSpace(serr.String()), err
}

func execute(ctx context.Context, method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool) error {
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
}

func (k *K8sClient) GetConfigMap(ctx context.Context, cmName, namespace string) (*corev1.ConfigMap, error) {
	cm, err := k.CoreV1().ConfigMaps(namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cm, nil
}

func (k *K8sClient) CreateConfigMap(ctx context.Context, cfg *corev1.ConfigMap) error {
	_, err := k.CoreV1().ConfigMaps(cfg.Namespace).Create(ctx, cfg, metav1.CreateOptions{})
	return err
}

func (k *K8sClient) UpdateConfigMap(ctx context.Context, cfg *corev1.ConfigMap) error {
	_, err := k.CoreV1().ConfigMaps(cfg.Namespace).Update(ctx, cfg, metav1.UpdateOptions{})
	return err
}

func (k *K8sClient) CreateEvent(ctx context.Context, pod corev1.Pod, evtType, reason, message string) error {
	now := time.Now()
	_, err := k.CoreV1().Events(pod.Namespace).Create(ctx, &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", pod.Name, now.UnixNano()),
			Namespace: pod.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:       pod.Kind,
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			UID:        pod.UID,
			APIVersion: pod.APIVersion,
		},
		Reason:  reason,
		Message: message,
		Source: corev1.EventSource{
			Component: "juicefs-csi-node",
			Host:      pod.Spec.NodeName,
		},
		FirstTimestamp:      metav1.Time{Time: now},
		LastTimestamp:       metav1.Time{Time: now},
		Type:                evtType,
		ReportingController: "juicefs-csi-node",
		ReportingInstance:   pod.Spec.NodeName,
	}, metav1.CreateOptions{})
	return err
}

func (k *K8sClient) GetEvents(ctx context.Context, pod *corev1.Pod) ([]corev1.Event, error) {
	events, err := k.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name), TypeMeta: metav1.TypeMeta{Kind: "Pod"}})
	if err != nil {
		return nil, err
	}
	return events.Items, nil
}

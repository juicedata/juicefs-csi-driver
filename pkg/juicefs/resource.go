package juicefs

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

func GetOrCreatePod(k8sClient *kubernetes.Clientset, volumeId, metaUrl string) (*corev1.Pod, error) {
	klog.V(5).Infof("Get pod of volumeId %s", volumeId)
	mntPod, err := k8sClient.CoreV1().Pods(Namespace).Get(context.TODO(), volumeId, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		// if not exist, create pod
		klog.V(5).Infof("Pod of volumeId %s does not exist, create it.", volumeId)
		mntPod = NewMountPod(volumeId, metaUrl)
		mntPod, err = k8sClient.CoreV1().Pods(Namespace).Create(context.TODO(), mntPod, metav1.CreateOptions{})
		if err != nil {
			klog.V(5).Infof("Can't create pod of volumeId %s: %v", volumeId, err)
			return nil, err
		}
		return mntPod, nil
	} else if err != nil {
		klog.V(5).Infof("Can't get pod of volumeId %s: %v", volumeId, err)
		return nil, err
	}
	return mntPod, nil
}

func GetPod(k8sClient *kubernetes.Clientset, volumeId string) (*corev1.Pod, error) {
	klog.V(5).Infof("Get pod of volumeId %s", volumeId)
	mntPod, err := k8sClient.CoreV1().Pods(Namespace).Get(context.TODO(), volumeId, metav1.GetOptions{})
	if err != nil {
		klog.V(5).Infof("Can't get pod of volumeId %s: %v", volumeId, err)
		return nil, err
	}
	return mntPod, nil
}

func DeletePod(k8sClient *kubernetes.Clientset, pod *corev1.Pod) error {
	klog.V(5).Infof("Delete pod %v", pod.Name)
	return k8sClient.CoreV1().Pods(Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
}

func GetOrCreateConfigMap(k8sClient *kubernetes.Clientset, volumeId string) (*corev1.ConfigMap, error) {
	klog.V(5).Infof("Get pod of volumeId %s", volumeId)
	cm, err := k8sClient.CoreV1().ConfigMaps(Namespace).Get(context.TODO(), volumeId, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		// if not exist, create cm
		klog.V(5).Infof("ConfigMap of volumeId %s does not exist, create it.", volumeId)
		cm = NewMountConfigMap(volumeId)
		cm, err = k8sClient.CoreV1().ConfigMaps(Namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
		if err != nil {
			klog.V(5).Infof("Can't create configMap of volumeId %s: %v", volumeId, err)
			return nil, err
		}
		return cm, nil
	} else if err != nil {
		klog.V(5).Infof("Can't get configMap of volumeId %s: %v", volumeId, err)
		return nil, err
	}
	return cm, nil
}

func GetConfigMap(k8sClient *kubernetes.Clientset, volumeId string) (*corev1.ConfigMap, error) {
	klog.V(5).Infof("Get configMap of volumeId %s", volumeId)
	cm, err := k8sClient.CoreV1().ConfigMaps(Namespace).Get(context.TODO(), volumeId, metav1.GetOptions{})
	if err != nil {
		klog.V(5).Infof("Can't get configMap of volumeId %s: %v", volumeId, err)
		return nil, err
	}
	return cm, nil
}

func UpdateConfigMap(k8sClient *kubernetes.Clientset, cm *corev1.ConfigMap) error {
	klog.V(5).Infof("Update configMap %v", cm.Name)
	_, err := k8sClient.CoreV1().ConfigMaps(Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	return err
}

func DeleteConfigMap(k8sClient *kubernetes.Clientset, cm *corev1.ConfigMap) error {
	klog.V(5).Infof("Delete configMap %v", cm.Name)
	return k8sClient.CoreV1().ConfigMaps(Namespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{})
}

func NewMountConfigMap(volumeId string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      volumeId,
			Namespace: Namespace,
			Labels: map[string]string{
				volumeId: volumeId,
			},
		},
		Data: map[string]string{},
	}
}

func NewMountPod(volumeId, metaUrl string) *corev1.Pod {
	isPrivileged := true
	mp := corev1.MountPropagationBidirectional
	dir := corev1.HostPathDirectory
	var pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      volumeId,
			Namespace: Namespace,
			Labels: map[string]string{
				VolumeId: volumeId,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "jfs-mount",
				Image:           MountImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{"sh", "-c", fmt.Sprintf("%v %v %v && sleep infinity",
					ceMountPath, metaUrl, MountPointPath)},
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
				},
				Resources: parsePodResources(),
				VolumeMounts: []corev1.VolumeMount{{
					Name:             "jfs-dir",
					MountPath:        "/jfs",
					MountPropagation: &mp,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "jfs-dir",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: MountPointPath,
						Type: &dir,
					},
				},
			}},
			NodeName: NodeName,
		},
	}
	return pod
}

func parsePodResources() corev1.ResourceRequirements {
	podLimit := corev1.ResourceList{}
	podRequest := corev1.ResourceList{}
	if MountPodCpuLimit != "" {
		podLimit.Cpu().Add(resource.MustParse(MountPodCpuLimit))
	}
	if MountPodMemLimit != "" {
		podLimit.Memory().Add(resource.MustParse(MountPodMemLimit))
	}
	if MountPodCpuRequest != "" {
		podRequest.Cpu().Add(resource.MustParse(MountPodCpuRequest))
	}
	if MountPodMemRequest != "" {
		podRequest.Memory().Add(resource.MustParse(MountPodMemRequest))
	}
	return corev1.ResourceRequirements{
		Limits:   podLimit,
		Requests: podRequest,
	}
}

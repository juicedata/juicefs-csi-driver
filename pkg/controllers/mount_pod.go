package controllers

import (
	"fmt"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const podMountRef string = "juicefs-mount"

func NewMountPod(instance *mountv1.JuiceMount) *corev1.Pod {
	secretEnv := corev1.EnvVar{
		Name: "metaurl",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				Key: "metaurl",
				LocalObjectReference: corev1.LocalObjectReference{
					Name: instance.Spec.MountSpec.Secret,
				},
			},
		},
	}
	isPrivileged := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", instance.Name),
			Namespace:    instance.Namespace,
			Labels: map[string]string{
				podMountRef: instance.Name,
			},
			OwnerReferences: []metav1.OwnerReference{{
				Kind: instance.Kind,
				UID:  instance.UID,
				Name: instance.Name,
			}},
			ManagedFields: nil,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "jfs-mount",
				Image:           instance.Spec.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{"sh", "-c", fmt.Sprintf("%v ${metaurl} %v && sleep infinity",
					instance.Spec.MountSpec.JuiceFsPath, instance.Spec.MountSpec.MountPath)},
				Env: []corev1.EnvVar{secretEnv},
				SecurityContext: &corev1.SecurityContext{
					Privileged: &isPrivileged,
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"SYS_ADMIN"},
					},
				},
				Lifecycle: &corev1.Lifecycle{
					PreStop: &corev1.Handler{
						Exec: &corev1.ExecAction{
							Command: []string{"umount", instance.Spec.MountSpec.MountPath},
						},
					},
				},
				Resources: corev1.ResourceRequirements{},
			}},
			NodeName: instance.Spec.NodeName,
		},
	}
}

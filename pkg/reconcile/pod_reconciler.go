package reconcile

import (
	"fmt"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (d *defaultResourceConciler) podReconcile(expected *corev1.Pod, current *corev1.Pod, reconcileStatus *Status) error {
	return ResourceDrive(Params{
		Client:     d.Client,
		Recorder:   d.Recorder,
		Expected:   expected,
		Reconciled: current,
		NeedCreate: func() bool {
			return reconcileStatus.Status.MountStatus == mountv1.JMountInit
		},
		Reconcile: func() {
			reconcilePod(*current, reconcileStatus)
		},
		PostCreate: func() {
			reconcileStatus.Status.MountStatus = mountv1.JMountRunning
			reconcileStatus.Events = append(reconcileStatus.Events, common.Event{
				EventType: "PodCreate",
				Reason:    fmt.Sprintf("pod of %s is created", d.JM.Name),
				Message:   fmt.Sprintf("pod of %s is created", d.JM.Name),
			})
		},
	})
}

func newMountPod(instance mountv1.JuiceMount) *corev1.Pod {
	isPrivileged := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", instance.Name),
			Namespace:    instance.Namespace,
			Labels: map[string]string{
				mountv1.PodMountRef: instance.Name,
			},
			OwnerReferences: []metav1.OwnerReference{{
				Kind: instance.Kind,
				UID:  instance.UID,
				Name: instance.Name,
			}},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            "jfs-mount",
				Image:           instance.Spec.MountSpec.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{"sh", "-c", fmt.Sprintf("%v %v %v && sleep infinity",
					instance.Spec.MountSpec.JuiceFsPath, instance.Spec.MountSpec.MetaUrl, instance.Spec.MountSpec.MountPath)},
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

func reconcilePod(mountPod corev1.Pod, reconcileStatus *Status) {
	switch mountPod.Status.Phase {
	case corev1.PodRunning:
		// update jfsMount instance status when pod is ready
		for _, cn := range mountPod.Status.ContainerStatuses {
			if cn.State.Running == nil {
				reconcileStatus.Status.MountStatus = mountv1.JMountRunning
				break
			}
		}
		reconcileStatus.Events = append(reconcileStatus.Events, common.Event{
			EventType: "PodCreate",
			Reason:    fmt.Sprintf("pod of %s is ready", reconcileStatus.Mount.Name),
			Message:   fmt.Sprintf("pod of %s is ready", reconcileStatus.Mount.Name),
		})
		reconcileStatus.Status.MountStatus = mountv1.JMountSuccess
		break
	case corev1.PodFailed:
	case corev1.PodUnknown:
	case corev1.PodReasonUnschedulable:
		// update jfsMount instance status when pod is error
		reconcileStatus.Status.MountStatus = mountv1.JMountFailed
		reconcileStatus.Events = append(reconcileStatus.Events, common.Event{
			EventType: "PodCreate",
			Reason:    fmt.Sprintf("pod of %s is error", reconcileStatus.Mount.Name),
			Message:   fmt.Sprintf("pod of %s is error: %v", reconcileStatus.Mount.Name, mountPod.Status.Message),
		})
		break
	}
}

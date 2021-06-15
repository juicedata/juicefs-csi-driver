package reconcile

import (
	"context"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceReconciler interface {
	Reconcile(context.Context) *common.Results
}

func NewResourceReconciler(parameters ResourceParameters) ResourceReconciler {
	return &defaultResourceConciler{ResourceParameters: parameters}
}

type ResourceParameters struct {
	JM  mountv1.JuiceMount
	Pod *corev1.Pod

	Client   client.Client
	Recorder record.EventRecorder

	ReconcileState *Status
}

type defaultResourceConciler struct {
	ResourceParameters
}

func (d *defaultResourceConciler) Reconcile(ctx context.Context) *common.Results {
	results := common.NewResults(ctx)

	err := d.fetchResourceFromK8sApi()
	if err != nil {
		return results.WithError(err)
	}
	expect := newMountPod(d.JM)
	err = d.podReconcile(expect, d.Pod, d.ReconcileState)
	if err != nil {
		klog.V(5).ErrorS(err, "Pod reconcile error",
			"namespace", d.JM.Namespace, "generate name", expect.GenerateName)
	}
	return results.WithError(err)
}

func GarbageCollectSoftOwnedResource(c client.Client, owner mountv1.JuiceMount) error {
	pods := &corev1.PodList{}

	err := c.List(context.Background(), pods,
		client.InNamespace(owner.Namespace),
		client.MatchingLabels{mountv1.PodMountRef: owner.Name})
	if err != nil {
		klog.V(5).ErrorS(err, "Select pods error",
			"labels", map[string]string{mountv1.PodMountRef: owner.Name})
		return err
	}
	for _, pod := range pods.Items {
		err = c.Delete(context.Background(), &pod)
		if err != nil {
			klog.V(5).ErrorS(err, "Pod delete error",
				"namespace", owner.Namespace, "name", pod.Name)
			return err
		}
	}
	return nil
}

func (d *defaultResourceConciler) fetchResourceFromK8sApi() error {
	pods := &corev1.PodList{}

	err := d.Client.List(context.Background(), pods,
		client.InNamespace(d.JM.Namespace),
		client.MatchingLabels{mountv1.PodMountRef: d.JM.Name})
	if err != nil {
		klog.V(5).ErrorS(err, "Select pods error", map[string]string{mountv1.PodMountRef: d.JM.Name})
		return err
	}
	if len(pods.Items) != 0 {
		d.Pod = &pods.Items[0]
	}
	return nil
}

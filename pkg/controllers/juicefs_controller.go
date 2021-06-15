/*
Copyright 2021.

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

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	reconciler "github.com/juicedata/juicefs-csi-driver/pkg/reconcile"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// JuicefsReconciler reconciles a Juicefs object
type JuicefsReconciler struct {
	Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type Client struct {
	client.Client
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=mount.juicefs.com,resources=juicemount,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mount.juicefs.com,resources=juicemount/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mount.juicefs.com,resources=juicemount/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the JuiceMount object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (j *JuicefsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	result := common.NewResults(ctx)
	// fetching jfsMount instance
	jfsMountInstance, err := j.fetchJuiceMount(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// todo CR check

	sts := reconciler.NewStatus(jfsMountInstance)
	internalResult := j.internalReconcile(ctx, jfsMountInstance, sts)
	err = j.updateStatus(ctx, jfsMountInstance, sts)
	return result.WithError(err).WithResult(internalResult).Aggregate()
}

func (j *JuicefsReconciler) fetchJuiceMount(ctx context.Context, name types.NamespacedName) (*mountv1.JuiceMount, error) {
	juiceMount := &mountv1.JuiceMount{}
	if err := j.Get(ctx, name, juiceMount); err != nil {
		j.Log.Error(err, "get juice mount cr failed", "namespace", name.Namespace, "name", name.Name)
		return nil, err
	}
	return juiceMount, nil
}

func (j *JuicefsReconciler) fetchPods(ctx context.Context, name types.NamespacedName) (*mountv1.JuiceMount, error) {
	juiceMount := &mountv1.JuiceMount{}
	if err := j.Get(ctx, name, juiceMount); err != nil {
		j.Log.Error(err, "get juice mount cr failed", "namespace", name.Namespace, "name", name.Name)
		return nil, err
	}
	return juiceMount, nil
}

// internal check
func (j *JuicefsReconciler) internalReconcile(ctx context.Context, juiceMount *mountv1.JuiceMount, status *reconciler.Status) *common.Results {
	results := common.NewResult(ctx)

	//deal with jm is deleted
	if juiceMount.IsMarkDeleted() {
		return results.WithError(j.onDelete(*juiceMount))
	}

	// todo mount check

	resourceParam := reconciler.ResourceParameters{
		JM:             *juiceMount,
		Client:         j.Client,
		Recorder:       j.Recorder,
		ReconcileState: status,
	}
	resourceReconciler := reconciler.NewResourceReconciler(resourceParam)
	resourceResult := resourceReconciler.Reconcile(ctx)
	return results.WithResult(resourceResult)
}

func (j *JuicefsReconciler) updateStatus(ctx context.Context, juiceMount *mountv1.JuiceMount, status *reconciler.Status) error {
	events, crt := status.Apply()
	if crt == nil {
		return nil
	}

	// record event to k8s
	for _, evt := range events {
		klog.V(5).InfoS("Record events", "event", evt)
		j.Recorder.Event(juiceMount, evt.EventType, evt.Reason, evt.Message)
	}

	// update status to k8s
	klog.V(5).InfoS("Update juiceMount status", "namespace", crt.Namespace, "name", crt.Name)
	return j.Client.Status().Update(ctx, crt)
}

func (j *JuicefsReconciler) onDelete(jm mountv1.JuiceMount) error {
	return reconciler.GarbageCollectSoftOwnedResource(j.Client, jm)
}

// SetupWithManager sets up the controller with the Manager.
func (j *JuicefsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mountv1.JuiceMount{}).Complete(j)
}

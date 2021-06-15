package reconcile

import (
	"context"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Params struct {
	Client   client.Client
	Recorder record.EventRecorder

	Expected   client.Object
	Reconciled client.Object

	NeedUpdate       func() bool
	UpdateReconciled func()
	NeedCreate       func() bool
	Reconcile        func()

	PostCreate func()
	PostUpdate func()
}

func ResourceDrive(params Params) error {
	namespace := params.Expected.GetNamespace()
	name := params.Expected.GetName()

	// if need create
	if params.NeedCreate() {
		klog.V(5).InfoS("Resource not exist, create it.",
			"namespace", namespace, "name", name)
		e := params.Client.Create(context.Background(), params.Expected)
		if e != nil {
			return e
		}
		if params.PostCreate != nil {
			params.PostCreate()
		}
	}

	// if need update
	if params.NeedUpdate != nil && params.NeedUpdate() {
		params.UpdateReconciled()
		return params.Client.Update(context.Background(), params.Reconciled)
	}

	// reconcile
	if params.Reconcile != nil {
		params.Reconcile()
	}
	return nil
}

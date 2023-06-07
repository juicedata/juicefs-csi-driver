/*
 Copyright 2023 Juicedata Inc

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

package controller

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

type JobController struct {
	*k8sclient.K8sClient
}

func NewJobController(client *k8sclient.K8sClient) *JobController {
	return &JobController{client}
}

func (m *JobController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.V(6).Infof("Receive job %s %s", request.Name, request.Namespace)
	job, err := m.GetJob(ctx, request.Name, request.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		klog.Errorf("get job %s error: %v", request.Name, err)
		return reconcile.Result{}, err
	}
	if job == nil {
		klog.V(6).Infof("job %s has been deleted.", request.Name)
		return reconcile.Result{}, nil
	}

	// check job deleted
	if job.DeletionTimestamp == nil {
		klog.V(6).Infof("job %s is not deleted", job.Name)
		return reconcile.Result{}, nil
	}

	// check csi node exist or not
	nodeName := job.Spec.Template.Spec.NodeName
	if nodeName == "" {
		// todo
	}
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{config.CSINodeLabelKey: config.CSINodeLabelValue},
	}
	fieldSelector := fields.Set{
		"spec.nodeName": nodeName,
	}
	csiPods, err := m.ListPod(ctx, config.Namespace, &labelSelector, &fieldSelector)
	if err != nil {
		klog.Errorf("list pod by label %s and field %s error: %v", config.CSINodeLabelValue, nodeName, err)
		return reconcile.Result{}, err
	}
	if len(csiPods) > 0 {
		klog.V(6).Infof("csi node in %s exists.", nodeName)
		return reconcile.Result{}, nil
	}

	klog.Infof("csi node in %s did not exist. delete job %s", nodeName, job.Name)
	err = m.DeleteJob(ctx, job.Name, job.Namespace)
	if err != nil {
		klog.Errorf("delete job %s error: %v", job.Name, err)
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, err
}

func (m *JobController) SetupWithManager(mgr ctrl.Manager) error {
	c, err := controller.New("mount", mgr, controller.Options{Reconciler: m})
	if err != nil {
		return err
	}

	return c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			job := event.Object.(*batchv1.Job)
			klog.V(6).Infof("watch job %s created", job.GetName())
			// check job deleted
			if job.DeletionTimestamp == nil {
				klog.V(6).Infof("job %s is not deleted", job.Name)
				return false
			}
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			jobNew, ok := updateEvent.ObjectNew.(*batchv1.Job)
			klog.V(6).Infof("watch job %s updated", jobNew.GetName())
			if !ok {
				klog.V(6).Infof("job.onUpdateFunc Skip object: %v", updateEvent.ObjectNew)
				return false
			}

			jobOld, ok := updateEvent.ObjectOld.(*batchv1.Job)
			if !ok {
				klog.V(6).Infof("job.onUpdateFunc Skip object: %v", updateEvent.ObjectOld)
				return false
			}

			if jobNew.GetResourceVersion() == jobOld.GetResourceVersion() {
				klog.V(6).Info("job.onUpdateFunc Skip due to resourceVersion not changed")
				return false
			}
			// check job deleted
			if jobNew.DeletionTimestamp == nil {
				klog.V(6).Infof("job %s is not deleted", jobNew.Name)
				return false
			}
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			job := deleteEvent.Object.(*batchv1.Job)
			klog.V(6).Infof("watch job %s deleted", job.GetName())
			// check job deleted
			if job.DeletionTimestamp == nil {
				klog.V(6).Infof("job %s is not deleted", job.Name)
				return false
			}
			return true
		},
	})
}

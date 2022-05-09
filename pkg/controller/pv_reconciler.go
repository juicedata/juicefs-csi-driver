/*
 Copyright 2022 Juicedata Inc

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
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
)

type PVReconciler struct {
	mount.SafeFormatAndMount
	Client  *k8sclient.K8sClient
	juicefs juicefs.Interface
}

func NewPVReconciler(k8sClient *k8sclient.K8sClient) *PVReconciler {
	mounter := mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      k8sexec.New(),
	}
	jfs := juicefs.NewJfsProvider(&mounter, k8sClient)
	return &PVReconciler{
		SafeFormatAndMount: mounter,
		Client:             k8sClient,
		juicefs:            jfs,
	}
}

func (p PVReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (p PVReconciler) SetupWithPVManager(mgr ctrl.Manager) error {
	c, err := controller.New("persistentvolume", mgr, controller.Options{
		Reconciler: p,
	})
	if err != nil {
		return err
	}
	return c.Watch(
		&source.Kind{Type: &corev1.PersistentVolume{}},
		&handler.EnqueueRequestForObject{}, predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) (onCreate bool) {
				// ignore create event
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) (needUpdate bool) {
				// ignore create event
				return false
			},
			DeleteFunc: p.DeleteFunc,
		},
	)
}

func (p PVReconciler) DeleteFunc(e event.DeleteEvent) bool {
	pv := e.Object.(*corev1.PersistentVolume)
	klog.V(6).Infof("Receive PersistentVolume deleted. %v", pv.Name)
	if pv.Spec.CSI != nil && pv.Spec.CSI.Driver != driver.DriverName {
		return false
	}
	if pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimDelete {
		return false
	}

	secretName, secretNamespace := pv.Spec.CSI.NodePublishSecretRef.Name, pv.Spec.CSI.NodePublishSecretRef.Namespace
	secret, err := p.Client.GetSecret(secretName, secretNamespace)
	if err != nil {
		klog.Errorf("[PVReconciler]: Get Secret error: %v", err)
		return false
	}
	secretData := make(map[string]string)
	for k, v := range secret.Data {
		secretData[k] = string(v)
	}
	volCtx := pv.Spec.CSI.VolumeAttributes

	mountOptions := []string{}
	// get mountOptions from PV.volumeAttributes or StorageClass.parameters
	if opts, ok := volCtx["mountOptions"]; ok {
		mountOptions = strings.Split(opts, ",")
	}
	if pv.Spec.MountOptions != nil {
		mountOptions = append(mountOptions, pv.Spec.MountOptions...)
	}

	klog.V(5).Infof("[PVReconciler]: clean cache of volume %s", pv.Name)
	if err := p.juicefs.JfsCleanupCache(pv.Spec.CSI.VolumeHandle, secretData, mountOptions); err != nil {
		klog.Errorf("[PVReconciler] clean up juicefs cache error: %s", err)
		return false
	}
	return false
}

/*
 Copyright 2024 Juicedata Inc

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

package main

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kdescribe "k8s.io/kubectl/pkg/describe"
)

func newPVCDescribe(clientSet *kubernetes.Clientset, pvc *corev1.PersistentVolumeClaim) (describeInterface, error) {
	if pvc == nil {
		return nil, fmt.Errorf("pvc not found")
	}
	describe := &pvcDescribe{
		pvc:       pvc,
		name:      pvc.Name,
		namespace: pvc.Namespace,
		status:    string(pvc.Status.Phase),
	}
	var (
		volumeId string
		err      error
	)
	if pvc.Spec.VolumeName != "" {
		describe.pv, err = clientSet.CoreV1().PersistentVolumes().Get(context.TODO(), pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
			describe.pv = nil
		}
		if describe.pv != nil && describe.pv.Spec.CSI != nil {
			volumeId = describe.pv.Spec.CSI.VolumeHandle
		}
		describe.pvName = pvc.Spec.VolumeName
	}
	if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
		describe.sc, err = clientSet.StorageV1().StorageClasses().Get(context.TODO(), *pvc.Spec.StorageClassName, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
			describe.sc = nil
		}
		describe.scName = *pvc.Spec.StorageClassName
	}
	if volumeId != "" {
		mountPods, err := GetMountPodList(clientSet, volumeId)
		if err != nil {
			return nil, err
		}
		mountMaps := make(map[string]string)
		for _, mount := range mountPods {
			mountMaps[mount.Spec.NodeName] = mount.Name
		}
		apps, err := GetAppPodList(clientSet, pvc.Namespace)
		if err != nil {
			return nil, err
		}
		for _, app := range apps {
			for _, volume := range app.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
					describe.appMountPair = append(describe.appMountPair, appMount{
						appName: app.Name,
						mount:   mountMaps[app.Spec.NodeName],
						node:    app.Spec.NodeName,
					})
					break
				}
			}
		}
	}

	return describe, nil
}

type pvcDescribe struct {
	pvc *corev1.PersistentVolumeClaim
	pv  *corev1.PersistentVolume
	sc  *storagev1.StorageClass

	name         string
	namespace    string
	status       string
	scName       string
	pvName       string
	appMountPair []appMount
	failedReason string
}

type appMount struct {
	appName string
	mount   string
	node    string
}

func (p *pvcDescribe) failed(reason string) {
	if p.failedReason == "" {
		p.failedReason = reason
	}
}

func (p *pvcDescribe) debug() describeInterface {
	if p.pvc.DeletionTimestamp != nil {
		return p.debugTerminatingPVC()
	}
	return p.debugRunningPVC()
}

func (p *pvcDescribe) debugTerminatingPVC() *pvcDescribe {
	if len(p.appMountPair) != 0 {
		p.failed("pvc is still mounted by pod")
	}
	return p
}

func (p *pvcDescribe) debugRunningPVC() *pvcDescribe {
	if p.status == string(corev1.ClaimBound) {
		return p
	}
	if p.scName != "" && p.sc == nil {
		p.failed(fmt.Sprintf("StorageClass %s not found", p.scName))
	}
	if p.sc != nil {
		p.failed("The corresponding PV is not automatically created. Please check the log of juicefs csi controller.")
	}
	if p.pv == nil {
		if p.pvName != "" {
			p.failed("No matching PV found")
		}
		if p.pvc.Spec.Selector == nil {
			p.failed("pvc selector is not set")
		}
	}
	return p
}

func (p *pvcDescribe) describe() (string, error) {
	return tabbedString(func(out io.Writer) error {
		w := kdescribe.NewPrefixWriter(out)
		w.Write(kdescribe.LEVEL_0, "Name:\t%s\n", p.name)
		w.Write(kdescribe.LEVEL_0, "Namespace:\t%s\n", p.namespace)
		w.Write(kdescribe.LEVEL_0, "Status:\t%s\n", p.status)
		w.Write(kdescribe.LEVEL_0, "Volume:\t%s\n", p.pvName)
		w.Write(kdescribe.LEVEL_0, "StorageClass:\t%s\n", p.scName)
		w.Write(kdescribe.LEVEL_0, "Used by:\n")
		if len(p.appMountPair) > 0 {
			w.Write(kdescribe.LEVEL_1, "AppPod\tMountPod\tNode\n")
			w.Write(kdescribe.LEVEL_1, "------\t--------\t----\n")
			for _, pair := range p.appMountPair {
				w.Write(kdescribe.LEVEL_1, "%s\t%s\t%s\n", pair.appName, pair.mount, pair.node)
			}
		}
		if p.failedReason != "" {
			w.Write(kdescribe.LEVEL_0, "Failed Reason:\n")
			w.Write(kdescribe.LEVEL_1, "%s\n", p.failedReason)
		}
		return nil
	})
}

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
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	kdescribe "k8s.io/kubectl/pkg/describe"
)

func newPVDescribe(clientSet *kubernetes.Clientset, pv *corev1.PersistentVolume) (describeInterface, error) {
	if pv == nil {
		return nil, fmt.Errorf("pv not found")
	}
	describe := &pvDescribe{
		pv:     pv,
		name:   pv.Name,
		status: getPVStatus(*pv),
	}
	var (
		volumeId  string
		namespace string
		pvcName   string
	)
	if pv.Spec.ClaimRef != nil {
		describe.pvc = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		namespace = pv.Spec.ClaimRef.Namespace
		pvcName = pv.Spec.ClaimRef.Name
	}
	if pv.Spec.CSI != nil {
		volumeId = pv.Spec.CSI.VolumeHandle
	}

	describe.sc = pv.Spec.StorageClassName

	if volumeId != "" {
		mountPods, err := GetMountPodList(clientSet, volumeId)
		if err != nil {
			return nil, err
		}
		mountMaps := make(map[string]string)
		for _, mount := range mountPods {
			mountMaps[mount.Spec.NodeName] = mount.Name
		}
		apps, err := GetAppPodList(clientSet, namespace)
		if err != nil {
			return nil, err
		}
		for _, app := range apps {
			for _, volume := range app.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
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

type pvDescribe struct {
	pv           *corev1.PersistentVolume
	name         string
	status       string
	pvc          string
	sc           string
	appMountPair []appMount
	failedReason string
}

var _ describeInterface = &pvDescribe{}

func (p *pvDescribe) failedf(reason string, args ...interface{}) {
	reason = fmt.Sprintf(reason, args...)
	if p.failedReason == "" {
		p.failedReason = reason
	}
}

func (p *pvDescribe) debug() describeInterface {
	if p.pv.DeletionTimestamp != nil {
		return p.debugTerminatingPV()
	}
	return p.debugRunningPV()
}

func (p *pvDescribe) debugRunningPV() describeInterface {
	switch p.pv.Status.Phase {
	case corev1.VolumeBound:
		fallthrough
	case corev1.VolumePending:
	case corev1.VolumeAvailable:
		p.failedf("waiting for pvc to bind")
	case corev1.VolumeReleased:
		p.failedf("the bound pvc was deleted, waiting for volumes to be recycled")
	case corev1.VolumeFailed:
		p.failedf("the volumes were failed to be recycled.")
	}
	return p
}

func (p *pvDescribe) debugTerminatingPV() describeInterface {
	if p.pv.DeletionTimestamp == nil {
		return p
	}
	if p.pv.Status.Phase == corev1.VolumeBound {
		p.failedf("waiting for pvc %s to be deleted", p.pvc)
	}
	return p
}

func (p *pvDescribe) describe() (string, error) {
	return tabbedString(func(out io.Writer) error {
		w := kdescribe.NewPrefixWriter(out)
		w.Write(kdescribe.LEVEL_0, "Name:\t%s\n", p.name)
		w.Write(kdescribe.LEVEL_0, "Status:\t%s\n", p.status)
		w.Write(kdescribe.LEVEL_0, "Claim:\t%s\n", p.pvc)
		w.Write(kdescribe.LEVEL_0, "StorageClass:\t%s\n", p.sc)
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

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

package util

import (
	"context"
	"regexp"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"

	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var (
	pattern = regexp.MustCompile(`\${\.PVC\.((labels|annotations)\.(.*?)|.*?)}`)
)

type PVCMetadata struct {
	data        map[string]string
	labels      map[string]string
	annotations map[string]string
}

func NewPVCMeta(pvc v1.PersistentVolumeClaim) *PVCMetadata {
	return &PVCMetadata{
		data: map[string]string{
			"name":      pvc.Name,
			"namespace": pvc.Namespace,
		},
		labels:      pvc.Labels,
		annotations: pvc.Annotations,
	}
}

func (meta *PVCMetadata) StringParser(str string) string {
	result := pattern.FindAllStringSubmatch(str, -1)
	for _, r := range result {
		switch r[2] {
		case "labels":
			str = strings.ReplaceAll(str, r[0], meta.labels[r[3]])
		case "annotations":
			str = strings.ReplaceAll(str, r[0], meta.annotations[r[3]])
		default:
			str = strings.ReplaceAll(str, r[0], meta.data[r[1]])
		}
	}
	return str
}

func CheckForSubPath(ctx context.Context, client *k8s.K8sClient, volume *v1.PersistentVolume, pathPattern string) (shouldDeleted bool, err error) {
	if pathPattern == "" {
		return true, nil
	}
	nowSubPath := volume.Spec.PersistentVolumeSource.CSI.VolumeAttributes["subPath"]
	sc := volume.Spec.StorageClassName

	if sc == "" {
		return false, nil
	}

	// get all pvs
	fieldSelector := fields.Set{
		"spec.storageClassName": sc,
	}
	pvs, err := client.ListPersistentVolumes(ctx, nil, &fieldSelector)
	if err != nil {
		return false, err
	}
	for _, pv := range pvs {
		if pv.Name == volume.Name || pv.DeletionTimestamp != nil {
			continue
		}
		subPath := pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes["subPath"]
		if subPath == nowSubPath {
			return false, nil
		}
	}
	return true, nil
}

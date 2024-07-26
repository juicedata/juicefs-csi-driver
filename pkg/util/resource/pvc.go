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

package resource

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var (
	pattern = regexp.MustCompile(`\${\.(PVC|pvc|node)\.((labels|annotations)\.(.*?)|.*?)}`)
)

type objectMetadata struct {
	data        map[string]string
	labels      map[string]string
	annotations map[string]string
}

type ObjectMeta struct {
	pvc, node *objectMetadata
}

func NewObjectMeta(pvc v1.PersistentVolumeClaim, node *v1.Node) *ObjectMeta {
	meta := &ObjectMeta{
		pvc: &objectMetadata{
			data: map[string]string{
				"name":      pvc.Name,
				"namespace": pvc.Namespace,
			},
			labels:      pvc.Labels,
			annotations: pvc.Annotations,
		},
		node: &objectMetadata{},
	}
	if node != nil {
		meta.node.data = map[string]string{
			"name":    node.Name,
			"podCIDR": node.Spec.PodCIDR,
		}
		meta.node.labels = node.Labels
		meta.node.annotations = node.Annotations
	}
	return meta
}

func (meta *ObjectMeta) StringParser(str string) string {
	result := pattern.FindAllStringSubmatch(str, -1)
	for _, r := range result {
		switch r[1] {
		case "PVC", "pvc":
			str = meta.pvc.stringParser(str, r)
		case "node":
			str = meta.node.stringParser(str, r)
		default:
		}
	}
	return str
}

func (meta *objectMetadata) stringParser(str string, matches []string) string {
	switch matches[3] {
	case "labels":
		return strings.ReplaceAll(str, matches[0], meta.labels[matches[4]])
	case "annotations":
		return strings.ReplaceAll(str, matches[0], meta.annotations[matches[4]])
	default:
		return strings.ReplaceAll(str, matches[0], meta.data[matches[2]])
	}
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
	pvs, err := client.ListPersistentVolumes(ctx, nil, nil)
	if err != nil {
		return false, err
	}
	for _, pv := range pvs {
		if pv.Name == volume.Name || pv.DeletionTimestamp != nil || pv.Spec.StorageClassName != sc {
			continue
		}
		subPath := pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes["subPath"]
		if subPath == nowSubPath {
			klog.V(6).Infof("PV %s uses the same subPath %s", pv.Name, subPath)
			return false, nil
		}
	}
	return true, nil
}

func (meta *ObjectMeta) ResolveSecret(str string, pvName string) string {
	resolved := os.Expand(str, func(k string) string {
		switch k {
		case "pvc.name":
			return meta.pvc.data["name"]
		case "pvc.namespace":
			return meta.pvc.data["namespace"]
		case "pv.name":
			return pvName
		}
		for ak, av := range meta.pvc.annotations {
			if k == "pvc.annotations['"+ak+"']" {
				return av
			}
		}
		klog.Errorf("Cannot resolve %s. replace it with an empty string", k)
		return ""
	})
	return resolved
}

func CheckForSecretFinalizer(ctx context.Context, client *k8s.K8sClient, volume *v1.PersistentVolume) (shouldRemoveFinalizer bool, err error) {
	sc := volume.Spec.StorageClassName
	secretNamespace := volume.Spec.PersistentVolumeSource.CSI.VolumeAttributes[config.ProvisionerSecretNamespace]
	secretName := volume.Spec.PersistentVolumeSource.CSI.VolumeAttributes[config.ProvisionerSecretName]
	if sc == "" || secretNamespace == "" || secretName == "" {
		klog.V(5).Infof("Cannot check for the secret, storageclass: %s, secretNamespace: %s, secretName: %s", sc, secretNamespace, secretName)
		return false, nil
	}
	// get all pvs
	pvs, err := client.ListPersistentVolumes(ctx, nil, nil)
	if err != nil {
		return false, err
	}
	for _, pv := range pvs {
		if pv.Name == volume.Name || pv.DeletionTimestamp != nil || pv.Spec.StorageClassName != sc {
			continue
		}
		pvSecretNamespace := pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes[config.ProvisionerSecretNamespace]
		pvSecretName := pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes[config.ProvisionerSecretName]
		// Cannot remove the secret if it is used by another pv
		if secretNamespace == pvSecretNamespace && secretName == pvSecretName {
			klog.V(5).Infof("PV %s uses the same secret %s/%s", pv.Name, pvSecretNamespace, pvSecretName)
			return false, nil
		}
	}
	return true, nil
}

func patchSecretFinalizer(ctx context.Context, client *k8s.K8sClient, secret *v1.Secret) error {
	f := secret.GetFinalizers()
	payload := []k8s.PatchListValue{{
		Op:    "replace",
		Path:  "/metadata/finalizers",
		Value: f,
	}}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		klog.Errorf("Parse json error: %v", err)
		return err
	}
	if err := client.PatchSecret(ctx, secret, payloadBytes, types.JSONPatchType); err != nil {
		klog.Errorf("Patch secret err:%v", err)
		return err
	}
	return nil
}

func AddSecretFinalizer(ctx context.Context, client *k8s.K8sClient, secret *v1.Secret, finalizer string) error {
	if controllerutil.ContainsFinalizer(secret, finalizer) {
		return nil
	}
	controllerutil.AddFinalizer(secret, finalizer)
	return patchSecretFinalizer(ctx, client, secret)
}

func RemoveSecretFinalizer(ctx context.Context, client *k8s.K8sClient, secret *v1.Secret, finalizer string) error {
	if !controllerutil.ContainsFinalizer(secret, finalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(secret, finalizer)
	return patchSecretFinalizer(ctx, client, secret)
}

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

package builder

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func Test_serverless_getCacheDirVolumes_ephemeral(t *testing.T) {
	storageClassName := "gp3"
	ephemeralStorage := resource.MustParse("30Gi")

	setting := &config.JfsSetting{
		CacheDirs:      []string{},
		CachePVCs:      []config.CachePVC{},
		CacheEphemeral: []*config.CacheEphemeral{},
		Attr: &config.PodAttr{
			Image: "juicedata/mount:ce-nightly",
		},
	}
	setting.CacheEphemeral = []*config.CacheEphemeral{
		{
			StorageClassName: &storageClassName,
			Storage:          ephemeralStorage,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Path:             "/var/jfsCache-ephemeral-0",
		},
	}

	r := &ServerlessBuilder{
		PodBuilder: PodBuilder{
			BaseBuilder: BaseBuilder{
				jfsSetting: setting,
				capacity:   0,
			},
		},
	}

	cacheVolumes, cacheVolumeMounts := r.genCacheDirVolumes()

	// verify the ephemeral volume and mount are created correctly
	foundEphemeralVolume := false
	foundEphemeralMount := false
	for _, v := range cacheVolumes {
		if v.Name == "cachedir-ephemeral-0" {
			foundEphemeralVolume = true
			if v.VolumeSource.Ephemeral == nil {
				t.Error("expected ephemeral volume source, got nil")
			}
			if v.VolumeSource.Ephemeral.VolumeClaimTemplate == nil {
				t.Error("expected VolumeClaimTemplate, got nil")
			}
			spec := v.VolumeSource.Ephemeral.VolumeClaimTemplate.Spec
			if *spec.StorageClassName != "gp3" {
				t.Errorf("expected storageClassName gp3, got %s", *spec.StorageClassName)
			}
			storageReq := spec.Resources.Requests[corev1.ResourceStorage]
			if storageReq.Cmp(ephemeralStorage) != 0 {
				t.Errorf("expected storage 30Gi, got %s", storageReq.String())
			}
			if len(spec.AccessModes) != 1 || spec.AccessModes[0] != corev1.ReadWriteOnce {
				t.Errorf("expected accessModes [ReadWriteOnce], got %v", spec.AccessModes)
			}
		}
	}
	for _, vm := range cacheVolumeMounts {
		if vm.Name == "cachedir-ephemeral-0" {
			foundEphemeralMount = true
			if vm.MountPath != "/var/jfsCache-ephemeral-0" {
				t.Errorf("expected mountPath /var/jfsCache-ephemeral-0, got %s", vm.MountPath)
			}
			if vm.ReadOnly {
				t.Error("expected ReadOnly=false for ephemeral cache mount")
			}
		}
	}
	if !foundEphemeralVolume {
		t.Error("ephemeral volume not found in serverless genCacheDirVolumes output")
	}
	if !foundEphemeralMount {
		t.Error("ephemeral volumeMount not found in serverless genCacheDirVolumes output")
	}

	// verify no hostPath volumes are created (serverless doesn't support hostPath)
	for _, v := range cacheVolumes {
		if v.VolumeSource.HostPath != nil {
			t.Errorf("unexpected hostPath volume in serverless builder: %s", v.Name)
		}
	}
}

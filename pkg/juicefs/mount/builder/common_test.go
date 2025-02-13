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

package builder

import (
	"reflect"
	"testing"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func TestGenMetadata(t *testing.T) {
	tests := []struct {
		name            string
		jfsSetting      *config.JfsSetting
		wantLabels      map[string]string
		wantAnnotations map[string]string
	}{
		{
			name: "test-delete-delay",
			jfsSetting: &config.JfsSetting{
				DeletedDelay: "10s",
				CleanCache:   true,
				Attr: &config.PodAttr{
					Labels: map[string]string{
						"label1": "value1",
					},
					Annotations: map[string]string{
						"annotation1": "value1",
					},
				},
				UUID:        "uuid1",
				UniqueId:    "unique1",
				HashVal:     "hash1",
				UpgradeUUID: "hash1",
			},
			wantLabels: map[string]string{
				"label1":                      "value1",
				common.PodTypeKey:             common.PodTypeValue,
				common.PodUniqueIdLabelKey:    "unique1",
				common.PodJuiceHashLabelKey:   "hash1",
				common.PodUpgradeUUIDLabelKey: "hash1",
			},
			wantAnnotations: map[string]string{
				"annotation1":             "value1",
				common.DeleteDelayTimeKey: "10s",
				common.CleanCache:         "true",
				common.JuiceFSUUID:        "uuid1",
				common.UniqueId:           "unique1",
			},
		},
		{
			name: "test-overwrite-inter-should-not-overwrite",
			jfsSetting: &config.JfsSetting{
				Attr: &config.PodAttr{
					Annotations: map[string]string{
						common.JuiceFSUUID: "uuid4",
					},
				},
				UUID:        "uuid3",
				UniqueId:    "unique3",
				HashVal:     "hash1",
				UpgradeUUID: "hash1",
			},
			wantLabels: map[string]string{
				common.PodTypeKey:             common.PodTypeValue,
				common.PodUniqueIdLabelKey:    "unique3",
				common.PodJuiceHashLabelKey:   "hash1",
				common.PodUpgradeUUIDLabelKey: "hash1",
			},
			wantAnnotations: map[string]string{
				common.JuiceFSUUID: "uuid3",
				common.UniqueId:    "unique3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLabels, gotAnnotations := GenMetadata(tt.jfsSetting)
			if !reflect.DeepEqual(gotLabels, tt.wantLabels) {
				t.Errorf("GenMetadata() gotLabels = %v, want %v", gotLabels, tt.wantLabels)
			}
			if !reflect.DeepEqual(gotAnnotations, tt.wantAnnotations) {
				t.Errorf("GenMetadata() gotAnnotations = %v, want %v", gotAnnotations, tt.wantAnnotations)
			}
		})
	}
}

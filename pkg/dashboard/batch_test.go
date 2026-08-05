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

package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
)

func TestNewUpgradeJobIncludesBatchUpgradeTimeoutEnv(t *testing.T) {
	originalNamespace := config.Namespace
	config.Namespace = "kube-system"
	t.Cleanup(func() {
		config.Namespace = originalNamespace
	})

	t.Setenv(batchUpgradeTimeoutEnv, "45s")

	job := NewUpgradeJob("job-1")
	envs := job.Spec.Template.Spec.Containers[0].Env

	timeoutEnv := ""
	for _, env := range envs {
		if env.Name == batchUpgradeTimeoutEnv {
			timeoutEnv = env.Value
			break
		}
	}

	assert.Equal(t, "45s", timeoutEnv)
}

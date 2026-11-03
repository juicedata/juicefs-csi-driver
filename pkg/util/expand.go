/*
 Copyright 2026 Juicedata Inc

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
	"regexp"
	"strings"

	v1 "k8s.io/api/core/v1"
)

// nodeTemplatePattern matches the ${.node.*} templates that can be resolved
// at mount time. The syntax and resolution semantics are identical to the
// StorageClass template expansion performed by the provisioner
// (pkg/util/resource.ObjectMeta.StringParser), but this helper works against
// the node object of the node running the CSI node plugin.
var nodeTemplatePattern = regexp.MustCompile(`\$\{\.node\.((labels|annotations)\.(.*?)|(.*?))\}`)

// ExpandNodeTemplates resolves ${.node.name}, ${.node.podCIDR},
// ${.node.labels.<key>} and ${.node.annotations.<key>} in s against node,
// returning s unchanged when it contains no templates.
//
// The provisioner expands these templates only once at PV creation time
// (pkg/driver/provisioner.go), baking the SelectedNode's values into the PV.
// For volumes whose Mount Pods may run on different nodes over their
// lifetime (e.g. a single PVC shared across heterogeneous GPU machines with
// different data-disk layouts), cache-dir and similar node-dependent options
// must instead be resolved when each Mount Pod is created, which is where
// this helper is called (pkg/config.ParseSettingWithNode).
//
// A missing label/annotation key resolves to an empty string, matching the
// provisioner behavior. Unknown template fields are left as-is.
func ExpandNodeTemplates(s string, node *v1.Node) string {
	if node == nil || !strings.Contains(s, "${.node.") {
		return s
	}
	for _, m := range nodeTemplatePattern.FindAllStringSubmatch(s, -1) {
		var replacement string
		switch m[2] {
		case "labels":
			replacement = node.Labels[m[3]]
		case "annotations":
			replacement = node.Annotations[m[3]]
		default:
			switch m[1] {
			case "name":
				replacement = node.Name
			case "podCIDR":
				replacement = node.Spec.PodCIDR
			default:
				// unknown field: leave the template untouched
				continue
			}
		}
		s = strings.ReplaceAll(s, m[0], replacement)
	}
	return s
}

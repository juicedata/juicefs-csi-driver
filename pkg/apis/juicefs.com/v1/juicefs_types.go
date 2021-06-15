/*
Copyright 2021.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// JuiceMountSpec defines the desired state of Juicefs
type JuiceMountSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	MountSpec MountSpec `json:"mount_spec"`
	NodeName  string    `json:"node_name"`
}

type MountSpec struct {
	Image       string `json:"image"`
	MetaUrl     string `json:"meta_url"`
	JuiceFsPath string `json:"juice_fs_path"`
	MountPath   string `json:"mount_path"`
}

// JuiceMountStatus defines the observed state of Juicefs
type JuiceMountStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	MountStatus JMountStatus `json:"mount_status"`
}

type JMountStatus string

const (
	JMountInit    JMountStatus = "init"
	JMountFailed  JMountStatus = "failed"
	JMountRunning JMountStatus = "running"
	JMountSuccess JMountStatus = "success"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// JuiceMount is the Schema for the juicefs API
type JuiceMount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JuiceMountSpec   `json:"spec,omitempty"`
	Status JuiceMountStatus `json:"status,omitempty"`
}

func (jm JuiceMount) IsMarkDeleted() bool {
	return !jm.DeletionTimestamp.IsZero()
}

//+kubebuilder:object:root=true

// JuiceMountList contains a list of Juicefs
type JuiceMountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JuiceMount `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JuiceMount{}, &JuiceMountList{})
}

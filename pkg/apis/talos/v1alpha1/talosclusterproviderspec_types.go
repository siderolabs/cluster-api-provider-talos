/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//TalosClusterControlPlaneSpec is the spec that defines info about cluster controlplane
type TalosClusterControlPlaneSpec struct {
	Count int `json:"count,omitempty"`
}

//TalosClusterPlatformSpec defines info about platform configs
type TalosClusterPlatformSpec struct {
	Type   string `json:"type,omitempty"`
	Config string `json:"config,omitempty"`
}

// TalosClusterProviderSpecStatus defines the observed state of TalosClusterProviderSpec
type TalosClusterProviderSpecStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TalosClusterProviderSpec is the Schema for the talosclusterproviderspecs API
// +k8s:openapi-gen=true
type TalosClusterProviderSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	ControlPlane TalosClusterControlPlaneSpec   `json:"controlplane,omitempty"`
	Platform     TalosClusterPlatformSpec       `json:"platform,omitempty"`
	Status       TalosClusterProviderSpecStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TalosClusterProviderSpecList contains a list of TalosClusterProviderSpec
type TalosClusterProviderSpecList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TalosClusterProviderSpec `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TalosClusterProviderSpec{}, &TalosClusterProviderSpecList{})
}

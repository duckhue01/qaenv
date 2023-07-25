/*
Copyright 2023.

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
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type QAEnvStatusCode string

var (
	Pending   QAEnvStatusCode = "pending"
	Allocated QAEnvStatusCode = "allocated"
	Deployed  QAEnvStatusCode = "deployed"
	Failed    QAEnvStatusCode = "failed"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// QAEnvSpec defines the desired state of QAEnv
type QAEnvSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of QAEnv. Edit qaenv_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// QAEnvStatus defines the observed state of QAEnv
type QAEnvStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Kustomization
	// +optional
	Kustomization *meta.NamespacedObjectReference `json:"kustomization,omitempty"`

	// ImageReflector
	// +optional
	ImageReflector *meta.NamespacedObjectReference `json:"imageReflector,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// QAEnv is the Schema for the qaenvs API
type QAEnv struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QAEnvSpec   `json:"spec,omitempty"`
	Status QAEnvStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QAEnvList contains a list of QAEnv
type QAEnvList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QAEnv `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QAEnv{}, &QAEnvList{})
}

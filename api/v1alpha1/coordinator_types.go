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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CoordinatorSpec defines the desired state of Coordinator
type CoordinatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ImageRepositories are fluxcd ImageRepository object name. Used to fetch the latest image tag and detect update in env.
	// +required
	ImageRepositories []string `json:"imageRepositories"`

	// Secret is the name of secret.
	// +required
	Secret string `json:"secret"`

	// GithubRepoURL is application repository owner that Coordinator will observer events.
	// +required
	GithubRepoOwner string `json:"githubRepoOwner"`

	// GithubRepoName is application repository name that Coordinator will observer events.
	// +required
	GithubRepoName string `json:"githubRepoName"`
}

// CoordinatorStatus defines the observed state of Coordinator
type CoordinatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Coordinator is the Schema for the coordinators API
type Coordinator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CoordinatorSpec   `json:"spec,omitempty"`
	Status CoordinatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CoordinatorList contains a list of Coordinator
type CoordinatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Coordinator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Coordinator{}, &CoordinatorList{})
}

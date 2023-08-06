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
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CoordinatorSpec defines the desired state of Coordinator
type CoordinatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Secret hold needed secret for coordinator
	// +required
	SecretRef meta.NamespacedObjectReference `json:"secretRef"`

	// GithubRepoURL is application repository owner that Coordinator will observer events.
	// +required
	GithubRepoOwner string `json:"githubRepoOwner"`

	// Services is a map with key is repo name and values are services in that repo. Service must be unique on all repositories.
	// +required
	Services map[string][]string `json:"services"`

	// QAEnvLimit is upper bound number of QAEnv
	// +required
	// QAEnvLimit int `json:"qaEnvLimit"`

	// Interval
	// +required
	Interval metav1.Duration `json:"interval"`

	// QAEnvSpec
	// +required
	QAEnvTemplate QAEnvTemplate `json:"qaEnvTemplate"`

	ProjectName string `json:"projectName"`

	// SourceRef
	// +required
	SourceRef kustomizev1.CrossNamespaceSourceReference `json:"sourceRef"`
}

type QAEnvTemplate struct {
	// Interval
	// +required
	Interval metav1.Duration `json:"interval"`

	// QaEnvironments
	// +required
	QaEnvs []int `json:"qaEnvs"`
}

// CoordinatorStatus defines the observed state of Coordinator
type CoordinatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	TicketMap map[string]*TicketMap `json:"ticketMap"`
	QaEnvs    map[string]bool       `json:"qaEnvs"`
}

type TicketMap struct {
	Status          QAEnvStatusCode        `json:"status"`
	QAEnvIndex      *string                `json:"qaEnvIndex,omitempty"`
	PullRequestsMap map[string]PullRequest `json:"pullRequestNames"`
}

type PullRequest struct {
	Name       int    `json:"name"`
	Repository string `json:"repository"`
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

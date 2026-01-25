/*
Copyright 2023 The Kubernetes Authors.

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

// PRFilter defines criteria for matching pull requests.
type PRFilter struct {
	// BranchMatch is a regex to match the base branch of the PR.
	// +optional
	BranchMatch string `json:"branchMatch,omitempty"`
}

// Cdk8sAppProxyTemplate defines the Cdk8sAppProxy to be generated for each PR.
type Cdk8sAppProxyTemplate struct {
	// Metadata allows setting labels and annotations on the generated Cdk8sAppProxy.
	// +optional
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the template for the Cdk8sAppProxySpec.
	Spec Cdk8sAppProxySpec `json:"spec"`
}

// Cdk8sAppProxyGeneratorSpec defines the desired state of Cdk8sAppProxyGenerator.
type Cdk8sAppProxyGeneratorSpec struct {
	// Source defines the repository to watch for pull requests.
	Source GitRepositorySpec `json:"source"`

	// Filters defines criteria for matching pull requests.
	// +optional
	Filters []PRFilter `json:"filters,omitempty"`

	// Template defines the Cdk8sAppProxy to be generated for each PR.
	Template Cdk8sAppProxyTemplate `json:"template"`

	// Path (optional) is the path within the repository where the cdk8s application is located.
	// This overrides any path set in the Source.
	// +optional
	Path string `json:"path,omitempty"`

	// PollInterval defines how often the generator should poll the Git provider for open PRs.
	// Defaults to 5 minutes.
	// +optional
	PollInterval *metav1.Duration `json:"pollInterval,omitempty"`
}

// Cdk8sAppProxyGeneratorStatus defines the observed state of Cdk8sAppProxyGenerator.
type Cdk8sAppProxyGeneratorStatus struct {
	// Conditions defines the current state of the Cdk8sAppProxyGenerator.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastPolledTime is the last time the Git provider was polled for PRs.
	// +optional
	LastPolledTime *metav1.Time `json:"lastPolledTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=capg

// Cdk8sAppProxyGenerator is the Schema for the cdk8sappproxygenerators API.
type Cdk8sAppProxyGenerator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Cdk8sAppProxyGeneratorSpec   `json:"spec,omitempty"`
	Status Cdk8sAppProxyGeneratorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// Cdk8sAppProxyGeneratorList contains a list of Cdk8sAppProxyGenerator.
type Cdk8sAppProxyGeneratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cdk8sAppProxyGenerator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cdk8sAppProxyGenerator{}, &Cdk8sAppProxyGeneratorList{})
}

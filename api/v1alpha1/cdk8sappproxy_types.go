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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// GitRepositorySpec defines the desired state of a Git repository source.
type GitRepositorySpec struct {
	// URL is the git repository URL.
	// If the Repository is private,
	// the URL needs to be provided in the form of 'git@github.com:...'
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Reference (optional) defines the branch, tag or hash which CAAPC
	// will pull from. If left empty, defaults to 'main'.
	// +kubebuilder:validation:optional
	Reference string `json:"reference,omitempty"`

	// Path (optional) is the path within the repository where the cdk8s application is located.
	// Defaults to the root of the repository.
	// +kubebuilder:validation:optional
	Path string `json:"path,omitempty"`

	// SecretRef references to a secret with the
	// needed token, used to pull from a private repository.
	// +kubebuilder:validation:optional
	SecretRef string `json:"secretRef,omitempty"`

	// SecretKey is the key within the SecretRef secret.
	// +kubebuilder:validation:optional
	SecretKey string `json:"secretKey,omitempty"`
}

// Cdk8sAppProxySpec defines the desired state of Cdk8sAppProxy.
type Cdk8sAppProxySpec struct {
	// GitRepository specifies the Git repository for the cdk8s app.
	// +kubebuilder:validation:Optional
	GitRepository *GitRepositorySpec `json:"gitRepository,omitempty"`

	// ClusterSelector selects the clusters to deploy the cdk8s app to.
	// +kubebuilder:validation:Required
	ClusterSelector metav1.LabelSelector `json:"clusterSelector"`
}

// Cdk8sAppProxyStatus defines the observed state of Cdk8sAppProxy.
type Cdk8sAppProxyStatus struct {
	// Conditions defines the current state of the Cdk8sAppProxy.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:resource:shortName=cap

// Cdk8sAppProxy is the Schema for the cdk8sappproxies API.
type Cdk8sAppProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   Cdk8sAppProxySpec   `json:"spec,omitempty"`
	Status Cdk8sAppProxyStatus `json:"status,omitempty"`
}

// GetConditions returns the list of conditions for an Cdk8sAppProxy API object.
func (c *Cdk8sAppProxy) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on an Cdk8sAppProxy API object.
func (c *Cdk8sAppProxy) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// Cdk8sAppProxyList contains a list of Cdk8sAppProxy.
type Cdk8sAppProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cdk8sAppProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cdk8sAppProxy{}, &Cdk8sAppProxyList{})
}

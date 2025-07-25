/*
Copyright 2022 The Kubernetes Authors.

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

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// Cdk8sAppProxy Conditions and Reasons.
const (
	// DeploymentProgressingCondition indicates that the cdk8s application deployment is progressing.
	DeploymentProgressingCondition clusterv1.ConditionType = "DeploymentProgressing"
	// GitCloneSuccessCondition indicates that the git clone operation was successful.
	GitCloneSuccessCondition = "GitCloneSuccess"
	// GitCloneFailedCondition indicates that git clone operation failed.
	GitCloneFailedCondition = "GitCloneFailed"
	// ValidGitRepositoryReason indicates that the given repository is valid.
	ValidGitRepositoryReason = "ValidGitRepository"
	// InvalidGitRepositoryReason indicates that the given repository is invalid.
	InvalidGitRepositoryReason = "InvalidGitRepository"
	// EmptyGitRepositoryReason indicates that no repository has been defined.
	EmptyGitRepositoryReason = "EmptyGitRepository"
	// GitHashSuccessReason indicates that the current commit hash was retrievable.
	GitHashSuccessReason = "GitHashSuccess"
	// GitHashFailureReason indicates that the current commit hash was not retrievable.
	GitHashFailureReason     = "GitHashFailure"
	GitOperationFailedReason = "GitOperationFailed"
	// Cdk8sSynthFailedReason indicates that cdk8s synth operation failed.
	Cdk8sSynthFailedReason = "Cdk8sSynthFailed"
	// WalkDistFailedReason indicates that walking the dist directory failed.
	WalkDistFailedReason = "WalkDistFailed"
	// NoManifestsFoundReason indicates that no YAML manifests were found in the dist directory.
	NoManifestsFoundReason = "NoManifestsFound"
	// ReadManifestFailedReason indicates that reading a manifest file failed.
	ReadManifestFailedReason = "ReadManifestFailed"
	// DecodeManifestFailedReason indicates that decoding YAML from a manifest file failed.
	DecodeManifestFailedReason = "DecodeManifestFailed"
	// DecodeToUnstructuredFailedReason indicates that decoding RawExtension to Unstructured failed.
	DecodeToUnstructuredFailedReason = "DecodeToUnstructuredFailed"
	// NoResourcesParsedReason indicates that no valid Kubernetes resources were parsed from manifest files.
	NoResourcesParsedReason = "NoResourcesParsed"
	// ClusterSelectorParseFailedReason indicates that parsing the ClusterSelector failed.
	ClusterSelectorParseFailedReason = "ClusterSelectorParseFailed"
	// ListClustersFailedReason indicates that listing clusters matching the selector failed.
	ListClustersFailedReason = "ListClustersFailed"
	// NoMatchingClustersReason indicates that no clusters were found matching the selector.
	NoMatchingClustersReason = "NoMatchingClusters"
	// KubeconfigUnavailableReason indicates that the kubeconfig for a target cluster is unavailable.
	KubeconfigUnavailableReason = "KubeconfigUnavailable"
	// ResourceApplyFailedReason indicates that applying a resource to a target cluster failed.
	ResourceApplyFailedReason = "ResourceApplyFailed"
	// GitAuthenticationFailedReason indicates that Git authentication failed (e.g., bad credentials).
	GitAuthenticationFailedReason string = "GitAuthenticationFailed"

	Cdk8sAppProxyReadyCondition clusterv1.ConditionType = "Cdk8sAppProxyReady"
)

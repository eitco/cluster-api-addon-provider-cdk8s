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
	// SynthCondition indicates that synthing the cdk8s code is progressing.
	SynthCondition = "SynthProgressing"
	// SynthFailedReason indicates that synthing the cdk8s code failed.
	SynthFailedReason = "SynthFailed"
	// ApplyResourcesCondition indicates that applying the resources is progressing.
	ApplyResourcesCondition = "ApplyingResources"
	// ApplyResourcesFailedReason indicates that applying the resources failed on the target cluster.
	ApplyResourcesFailedReason = "ApplyingResourcesFailed"
	// GitCloneCondition indicates that the cloning of a git repository is progressing.
	GitCloneCondition = "GitCloningProgressing"
	// GitCloneFailedReason indicates that the cloning of the git repository failed.
	GitCloneFailedReason = "GitCloneFailed"
)

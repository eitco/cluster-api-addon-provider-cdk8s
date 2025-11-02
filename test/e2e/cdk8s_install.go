//go:build e2e
// +build e2e

/*
Copyright 2024 The Kubernetes Authors.

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

package e2e

import (
	"context"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api/test/framework"
)

// Cdk8sInstallInput specifies the input for installing a Cdk8s chart on a workload cluster and verifying that it was successful.
type Cdk8sInstallInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	Cdk8sAppProxy         *addonsv1alpha1.Cdk8sAppProxy
}

// Cdk8sInstallSpec implements a test that verifies a Cdk8s chart can be installed on a workload cluster. It creates a Cdk8sAppProxy
// resource and patches the Cluster labels such that they match the Cdk8sAppProxies clusterSelector.
func Cdk8sInstallSpec(ctx context.Context, inputGetter func() Cdk8sInstallInput) {
	var (
		specName   = "cdk8s-install"
		input      Cdk8sInstallInput
		mgmtClient ctrlclient.Client
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)

	By("creating a Kubernetes client to the management cluster")
	mgmtClient = input.BootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	// Create CAP on management Cluster
	Byf("Creating Cdk8sAppProxy %s/%s", input.Namespace, input.Cdk8sAppProxy.Name)
	Expect(mgmtClient.Create(ctx, input.Cdk8sAppProxy)).To(Succeed())

	EnsureCdk8sAppProxyInstallOrUpgrade(ctx, specName, input.BootstrapClusterProxy, &input, nil, true)
}

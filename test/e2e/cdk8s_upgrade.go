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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api/test/framework"
)

// Cdk8sUpgradeInput specifies the input for updating or reinstalling a Cdk8s chart on a workload cluster and verifying that it was successful.
type Cdk8sUpgradeInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	Cdk8sAppProxy         *addonsv1alpha1.Cdk8sAppProxy // Note: Only the Spec field is used.
	ExpectedRevision      int
}

// Cdk8sUpgradeSpec implements a test that verifies a Cdk8s chart can be either updated or reinstalled on a workload cluster, depending on
// if an immutable field has changed. It takes a Cdk8sAppProxy resource and updates ONLY the spec field and patches the Cluster labels
// such that they match the Cdk8sAppProxy's clusterSelector. It then waits for the Cdk8s release to be deployed on the workload cluster.
func Cdk8sUpgradeSpec(ctx context.Context, inputGetter func() Cdk8sUpgradeInput) {
	var (
		specName   = "cdk8s-upgrade"
		input      Cdk8sUpgradeInput
		mgmtClient ctrlclient.Client
		err        error
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)

	By("creating a Kubernetes client to the management cluster")
	mgmtClient = input.BootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	// Get existing HCP from management Cluster
	existing := &addonsv1alpha1.Cdk8sAppProxy{}
	key := types.NamespacedName{
		Namespace: input.Cdk8sAppProxy.Namespace,
		Name:      input.Cdk8sAppProxy.Name,
	}
	err = mgmtClient.Get(ctx, key, existing)
	Expect(err).NotTo(HaveOccurred())

	// Patch HCP on management Cluster
	Byf("Patching Cdk8sAppProxy %s/%s", existing.Namespace, existing.Name)
	patchHelper, err := patch.NewHelper(existing, mgmtClient)
	Expect(err).ToNot(HaveOccurred())

	existing.Spec = input.Cdk8sAppProxy.Spec
	input.Cdk8sAppProxy = existing

	Eventually(func() error {
		return patchHelper.Patch(ctx, existing)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to patch Cdk8sAppProxy %s", klog.KObj(existing))

	EnsureCdk8sAppProxyInstallOrUpgrade(ctx, specName, input.BootstrapClusterProxy, nil, &input, true)
}

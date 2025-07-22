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
	"fmt"
	"os"
	"path/filepath"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"time"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Workload cluster creation", func() {
	var (
		ctx               = context.Background()
		specName          = "create-workload-cluster"
		namespace         *corev1.Namespace
		cancelWatches     context.CancelFunc
		result            *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName       string
		clusterNamePrefix string
		additionalCleanup func()
		specTimes         = map[string]time.Time{}
		//installOnceWaitPeriod = 3 * time.Minute
		//numOutofBandUpgrades  = 5
	)

	BeforeEach(func() {
		logCheckpoint(specTimes)

		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(e2eConfig).NotTo(BeNil(), "e2eConfig is required for %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0o755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)
		Expect(e2eConfig.Variables).To(HaveKey(capi_e2e.KubernetesVersion))

		// CLUSTER_NAME and CLUSTER_NAMESPACE allows for testing existing clusters.
		// If CLUSTER_NAMESPACE is set, don't generate a new prefix. Otherwise,
		// the correct namespace won't be found and a new cluster will be created.
		clusterNameSpace := os.Getenv("CLUSTER_NAMESPACE")
		if clusterNameSpace == "" {
			clusterNamePrefix = fmt.Sprintf("caapc-e2e-%s", util.RandomString(6))
		} else {
			clusterNamePrefix = clusterNameSpace
		}

		// Set up a Namespace where to host objects for this spec and create a watcher for the namespace events.
		var err error
		namespace, cancelWatches, err = setupSpecNamespace(ctx, clusterNamePrefix, bootstrapClusterProxy, artifactFolder)
		Expect(err).NotTo(HaveOccurred())

		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		additionalCleanup = nil
	})

	AfterEach(func() {
		if result.Cluster == nil {
			// this means the cluster failed to come up. We make an attempt to find the cluster to be able to fetch logs for the failed bootstrapping.
			_ = bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{Name: clusterName, Namespace: namespace.Name}, result.Cluster)
		}

		CheckTestBeforeCleanup()

		cleanInput := cleanupInput{
			SpecName:          specName,
			Cluster:           result.Cluster,
			ClusterProxy:      bootstrapClusterProxy,
			Namespace:         namespace,
			CancelWatches:     cancelWatches,
			IntervalsGetter:   e2eConfig.GetIntervals,
			SkipCleanup:       skipCleanup,
			SkipLogCollection: skipLogCollection,
			AdditionalCleanup: additionalCleanup,
			ArtifactFolder:    artifactFolder,
		}
		dumpSpecResourcesAndCleanup(ctx, cleanInput)

		logCheckpoint(specTimes)
	})

	Context("Creating workload cluster [REQUIRED]", func() {
		It("With default template to install, upgrade, and uninstall a cdk8sappproxy", func() {
			clusterName = fmt.Sprintf("%s-%s", specName, util.RandomString(6))
			clusterctl.ApplyClusterTemplateAndWait(ctx, createApplyClusterTemplateInput(
				specName,
				withNamespace(namespace.Name),
				withClusterName(clusterName),
				withControlPlaneMachineCount(1),
				withWorkerMachineCount(1),
				withControlPlaneWaiters(clusterctl.ControlPlaneWaiters{
					WaitForControlPlaneInitialized: EnsureControlPlaneInitialized,
				}),
			), result)

			cdk8sap := &addonsv1alpha1.Cdk8sAppProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cdk8s-sample-app-go",
					Namespace: namespace.Name,
				},
				Spec: addonsv1alpha1.Cdk8sAppProxySpec{
					GitRepository: &addonsv1alpha1.GitRepositorySpec{
						URL:                   "https://github.com/PatrickLaabs/cdk8s-sample-deployment",
						Reference:             "main",
						Path:                  ".",
						ReferencePollInterval: &metav1.Duration{Duration: 30 * time.Second},
					},
					ClusterSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
				},
			}

			// Create new Cdk8sAppProxy to install cdk8s deployments.
			By("Creating new Cdk8sAppProxy to install cdk8s deployments", func() {
				Cdk8sInstallSpec(ctx, func() Cdk8sInstallInput {
					return Cdk8sInstallInput{
						BootstrapClusterProxy: bootstrapClusterProxy,
						Namespace:             namespace,
						ClusterName:           clusterName,
						Cdk8sAppProxy:         cdk8sap,
					}
				})
			})

			// ToDo: Need to implement the uninstallSpec to remove the Cdk8sAppProxy from the cluster.
			// Uninstall Cdk8sAppProxy by removing the label selector from the Cluster.
			//By("Uninstalling Cdk8sAppProxy from cluster", func() {
			//	Cdk8sUninstallSpec(ctx, func() Cdk8sUninstallInput {
			//		return Cdk8sUninstallInput{
			//			BootstrapClusterProxy: bootstrapClusterProxy,
			//			Namespace:             namespace,
			//			ClusterName:           clusterName,
			//			Cdk8sAppProxy:         cdk8sap,
			//		}
			//	})
			//})
		})
	})
})

type cleanupInput struct {
	SpecName          string
	ClusterProxy      framework.ClusterProxy
	ArtifactFolder    string
	Namespace         *corev1.Namespace
	CancelWatches     context.CancelFunc
	Cluster           *clusterv1.Cluster
	IntervalsGetter   func(spec, key string) []interface{}
	SkipCleanup       bool
	SkipLogCollection bool
	AdditionalCleanup func()
}

func dumpSpecResourcesAndCleanup(ctx context.Context, input cleanupInput) {
	defer func() {
		input.CancelWatches()
	}()

	Logf("Dumping all the Cluster API resources in the %q namespace", input.Namespace.Name)
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    input.ClusterProxy.GetClient(),
		Namespace: input.Namespace.Name,
		LogPath:   filepath.Join(input.ArtifactFolder, "clusters", input.ClusterProxy.GetName(), "resources"),
	})

	if input.Cluster == nil {
		By("Unable to dump workload cluster logs as the cluster is nil")
	} else if !input.SkipLogCollection {
		Byf("Dumping logs from the %q workload cluster", input.Cluster.Name)
		input.ClusterProxy.CollectWorkloadClusterLogs(ctx, input.Cluster.Namespace, input.Cluster.Name, filepath.Join(input.ArtifactFolder, "clusters", input.Cluster.Name))
	}

	if input.SkipCleanup {
		return
	}

	Logf("Deleting all clusters in the %s namespace", input.Namespace.Name)
	// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
	// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
	// instead of DeleteClusterAndWait
	deleteTimeoutConfig := "wait-delete-cluster"
	framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
		Client:    input.ClusterProxy.GetClient(),
		Namespace: input.Namespace.Name,
	}, input.IntervalsGetter(input.SpecName, deleteTimeoutConfig)...)

	Logf("Deleting namespace used for hosting the %q test spec", input.SpecName)
	framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
		Deleter: input.ClusterProxy.GetClient(),
		Name:    input.Namespace.Name,
	})

	if input.AdditionalCleanup != nil {
		Logf("Running additional cleanup for the %q test spec", input.SpecName)
		input.AdditionalCleanup()
	}
}

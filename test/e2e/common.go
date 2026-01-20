//go:build e2e
// +build e2e

/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 1.0 (the "License");
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
	"io"
	"log"
	"net/http"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	kubeadmv1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capi_e2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubesystem = "kube-system"
)

// EnsureControlPlaneInitialized waits for the cluster KubeadmControlPlane object to be initialized
// Fulfills the clusterctl.Waiter type so that it can be used as ApplyClusterTemplateAndWaitInput data
// in the flow of a clusterctl.ApplyClusterTemplateAndWait E2E test scenario.
func EnsureControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
	getter := input.ClusterProxy.GetClient()
	cluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
		Getter:    getter,
		Name:      input.ClusterName,
		Namespace: input.Namespace,
	})
	kubeadmControlPlane := &kubeadmv1.KubeadmControlPlane{}
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}

	By("Ensuring KubeadmControlPlane is initialized")
	Eventually(func(g Gomega) {
		g.Expect(getter.Get(ctx, key, kubeadmControlPlane)).To(Succeed(), "Failed to get KubeadmControlPlane object %s/%s", cluster.Namespace, cluster.Spec.ControlPlaneRef.Name)
		g.Expect(conditions.IsTrue(kubeadmControlPlane, kubeadmv1.KubeadmControlPlaneInitializedCondition)).To(BeTrue(), "KubeadmControlPlane is not yet initialized")
	}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "KubeadmControlPlane object %s/%s was not initialized in time", cluster.Namespace, cluster.Spec.ControlPlaneRef.Name)

	By("Installing CNI")
	EnsureCNIInstallation(ctx, CniInstallInput{
		BootstrapClusterProxy: input.ClusterProxy,
		ManifestUrl:           "https://raw.githubusercontent.com/projectcalico/calico/v3.27.0/manifests/calico.yaml",
		Namespace:             input.Namespace,
		ClusterName:           input.ClusterName,
	})

	By("Ensuring API Server is reachable before querying Cdk8sAppProxies")
	Eventually(func(g Gomega) {
		ns := &corev1.Namespace{}
		clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
		g.Expect(clusterProxy.GetClient().Get(ctx, client.ObjectKey{Name: kubesystem}, ns)).To(Succeed(), "Failed to get kube-system namespace")
	}, input.WaitForControlPlaneIntervals...).Should(Succeed(), "API Server was not reachable in time")

	// ToDo: We want to make sure, that calico is ready before we continue with the Cdk8sAppProxy installation.
	// By("Ensure calico is ready after control plane is initialized")
	// EnsureCalicoIsReady(ctx, input)

	result.ControlPlane = framework.DiscoveryAndWaitForControlPlaneInitialized(ctx, framework.DiscoveryAndWaitForControlPlaneInitializedInput{
		Lister:  input.ClusterProxy.GetClient(),
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

// ToDo: Guess we watch for the wrong namespaces here.
const (
	CalicoSystemNamespace    string = "calico-system"
	CalicoAPIServerNamespace string = "calico-apiserver"
)

// EnsureCNIInstallation ensures that the CNI is installed on the bootstrap cluster
func EnsureCNIInstallation(ctx context.Context, input CniInstallInput) {
	By("Installing CNI on workload cluster")
	workloadClusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())

	workloadClient := workloadClusterProxy.GetClient()
	Expect(workloadClient).NotTo(BeNil())

	// Fetch the manifest
	resp, err := http.Get(input.ManifestUrl)
	Expect(err).NotTo(HaveOccurred(), "Failed to fetch CNI manifest")
	defer resp.Body.Close()

	// Read and decode the YAML manifest
	decoder := yaml.NewYAMLOrJSONDecoder(resp.Body, 4096)
	for {
		// Create a new unstructured object for each resource in the YAML
		obj := &unstructured.Unstructured{}
		if err := decoder.Decode(obj); err != nil {
			if err == io.EOF {
				break
			}
			Expect(err).NotTo(HaveOccurred(), "Failed to decode CNI manifest")
		}

		// Skip empty objects
		if len(obj.Object) == 0 {
			continue
		}

		// Create the resource
		err = workloadClient.Create(ctx, obj)
		if err != nil {
			fmt.Printf("Warning: failed to create resource %s/%s: %v\n", obj.GetNamespace(), obj.GetName(), err)
		}
	}
}

// EnsureCalicoIsReady verifies that the calico deployments exist and are available on the workload cluster.
func EnsureCalicoIsReady(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput) {
	specName := "ensure-calico"

	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)

	By("Waiting for Ready calico-system deployment pods")
	for _, d := range []string{"calico-kube-controllers", "calico-typha"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, CalicoSystemNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
	By("Waiting for Ready calico-apiserver deployment pods")
	for _, d := range []string{"calico-apiserver"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, CalicoAPIServerNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
}

// EnsureCdk8sAppProxyInstallOrUpgrade ensures that a Cdk8s Install is successful. Only one of installInput or upgradeInput should be provided
func EnsureCdk8sAppProxyInstallOrUpgrade(ctx context.Context, specName string, bootstrapClusterProxy framework.ClusterProxy, installInput *Cdk8sInstallInput, upgradeInput *Cdk8sUpgradeInput, patchCluster bool) {
	var (
		clusterName      string
		clusterNamespace string
		cdk8sAppProxy    *addonsv1alpha1.Cdk8sAppProxy
		expectedRevision int
	)

	Expect(installInput != nil || upgradeInput != nil).To(BeTrue(), "either installInput or upgradeInput should be provided")
	if installInput != nil {
		Expect(upgradeInput).To(BeNil(), "only one of installInput or upgradeInput should be provided")
		clusterName = installInput.ClusterName
		clusterNamespace = installInput.Namespace.Name
		cdk8sAppProxy = installInput.Cdk8sAppProxy
		expectedRevision = 1
	} else if upgradeInput != nil {
		Expect(installInput).To(BeNil(), "only one of installInput or upgradeInput should be provided")
		clusterName = upgradeInput.ClusterName
		clusterNamespace = upgradeInput.Namespace.Name
		cdk8sAppProxy = upgradeInput.Cdk8sAppProxy
		expectedRevision = upgradeInput.ExpectedRevision
	}

	if patchCluster {
		mgmtClient := bootstrapClusterProxy.GetClient()
		Expect(mgmtClient).NotTo(BeNil())

		// Get Cluster from management Cluster
		workloadCluster := &clusterv1.Cluster{}
		key := apitypes.NamespacedName{
			Namespace: clusterNamespace,
			Name:      clusterName,
		}
		err := mgmtClient.Get(ctx, key, workloadCluster)
		Expect(err).NotTo(HaveOccurred())

		// Patch cluster labels, ignore match expressions for now
		selector := cdk8sAppProxy.Spec.ClusterSelector
		labels := workloadCluster.Labels
		if labels == nil {
			labels = make(map[string]string)
		}

		for k, v := range selector.MatchLabels {
			labels[k] = v
		}

		err = mgmtClient.Update(ctx, workloadCluster)
		Expect(err).NotTo(HaveOccurred())
	}

	capWaitInput := GetWaitForCdk8sAppProxyReadyInput(ctx, bootstrapClusterProxy, clusterName, cdk8sAppProxy, expectedRevision, specName)
	WaitForCdk8sAppProxyReady(ctx, capWaitInput, e2eConfig.GetIntervals(specName, "wait-cdk8s-app-proxy-ready")...)

	// Get workload Cluster proxy
	By("creating a clusterctl proxy to the workload cluster")
	workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, clusterNamespace, clusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())
}

// CheckTestBeforeCleanup checks to see if the current running Ginkgo test failed, and prints
// a status message regarding cleanup.
func CheckTestBeforeCleanup() {
	if CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
		Logf("FAILED!")
	}
	Logf("Cleaning up after \"%s\" spec", CurrentSpecReport().FullText())
}

func setupSpecNamespace(ctx context.Context, namespaceName string, clusterProxy framework.ClusterProxy, artifactFolder string) (*corev1.Namespace, context.CancelFunc, error) {
	Byf("Creating namespace %q for hosting the cluster", namespaceName)
	Logf("starting to create namespace for hosting the %q test spec", namespaceName)
	logPath := filepath.Join(artifactFolder, "clusters", clusterProxy.GetName())
	namespace, err := GetNamespace(ctx, clusterProxy.GetClientSet(), namespaceName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}

	// namespace exists wire it up
	if err == nil {
		Byf("Creating event watcher for existing namespace %q", namespace.Name)
		watchesCtx, cancelWatches := context.WithCancel(ctx)
		go func() {
			defer GinkgoRecover()
			framework.WatchNamespaceEvents(watchesCtx, framework.WatchNamespaceEventsInput{
				ClientSet: clusterProxy.GetClientSet(),
				Name:      namespace.Name,
				LogFolder: logPath,
			})
		}()

		return namespace, cancelWatches, nil
	}

	// create and wire up namespace
	namespace, cancelWatches := framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   clusterProxy.GetClient(),
		ClientSet: clusterProxy.GetClientSet(),
		Name:      namespaceName,
		LogFolder: logPath,
	})

	return namespace, cancelWatches, nil
}

// GetNamespace returns a namespace for with a given name
func GetNamespace(ctx context.Context, clientset *kubernetes.Clientset, name string) (*corev1.Namespace, error) {
	opts := metav1.GetOptions{}
	namespace, err := clientset.CoreV1().Namespaces().Get(ctx, name, opts)
	if err != nil {
		log.Printf("failed trying to get namespace (%s):%s\n", name, err.Error())
		return nil, err
	}

	return namespace, nil
}

func createApplyClusterTemplateInput(specName string, changes ...func(*clusterctl.ApplyClusterTemplateAndWaitInput)) clusterctl.ApplyClusterTemplateAndWaitInput {
	input := clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy: bootstrapClusterProxy,
		ConfigCluster: clusterctl.ConfigClusterInput{
			LogFolder:                filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName()),
			ClusterctlConfigPath:     clusterctlConfigPath,
			KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
			InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
			Flavor:                   clusterctl.DefaultFlavor,
			Namespace:                "default",
			ClusterName:              "cluster",
			KubernetesVersion:        e2eConfig.MustGetVariable(capi_e2e.KubernetesVersion),
			ControlPlaneMachineCount: ptr.To[int64](1),
			WorkerMachineCount:       ptr.To[int64](1),
		},
		WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
		WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
		WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
		WaitForMachinePools:          e2eConfig.GetIntervals(specName, "wait-machine-pool-nodes"),
		CNIManifestPath:              "",
	}
	for _, change := range changes {
		change(&input)
	}

	return input
}

func withClusterProxy(proxy framework.ClusterProxy) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ClusterProxy = proxy
	}
}

func withFlavor(flavor string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.Flavor = flavor
	}
}

func withNamespace(namespace string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.Namespace = namespace
	}
}

func withClusterName(clusterName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.ClusterName = clusterName
	}
}

func withKubernetesVersion(version string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.KubernetesVersion = version
	}
}

func withControlPlaneMachineCount(count int64) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.ControlPlaneMachineCount = ptr.To[int64](count)
	}
}

func withWorkerMachineCount(count int64) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ConfigCluster.WorkerMachineCount = ptr.To[int64](count)
	}
}

func withClusterInterval(specName string, intervalName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		if intervalName != "" {
			input.WaitForClusterIntervals = e2eConfig.GetIntervals(specName, intervalName)
		}
	}
}

func withControlPlaneInterval(specName string, intervalName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		if intervalName != "" {
			input.WaitForControlPlaneIntervals = e2eConfig.GetIntervals(specName, intervalName)
		}
	}
}

func withMachineDeploymentInterval(specName string, intervalName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		if intervalName != "" {
			input.WaitForMachineDeployments = e2eConfig.GetIntervals(specName, intervalName)
		}
	}
}

func withMachinePoolInterval(specName string, intervalName string) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		if intervalName != "" {
			input.WaitForMachinePools = e2eConfig.GetIntervals(specName, intervalName)
		}
	}
}

func withControlPlaneWaiters(waiters clusterctl.ControlPlaneWaiters) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.ControlPlaneWaiters = waiters
	}
}

func withPostMachinesProvisioned(postMachinesProvisioned func()) func(*clusterctl.ApplyClusterTemplateAndWaitInput) {
	return func(input *clusterctl.ApplyClusterTemplateAndWaitInput) {
		input.PostMachinesProvisioned = postMachinesProvisioned
	}
}

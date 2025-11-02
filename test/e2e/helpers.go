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
	"encoding/json"
	"fmt"
	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"strings"
	"text/tabwriter"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	sshPort                               = "22"
	deleteOperationTimeout                = 20 * time.Minute
	retryableOperationTimeout             = 30 * time.Second
	retryableOperationInterval            = 3 * time.Second
	retryableDeleteOperationTimeout       = 3 * time.Minute
	retryableOperationSleepBetweenRetries = 3 * time.Second
	cdk8sInstallTimeout                   = 3 * time.Minute
	sshConnectionTimeout                  = 30 * time.Second
)

func Byf(format string, a ...any) {
	By(fmt.Sprintf(format, a...))
}

// deploymentsClientAdapter adapts a Deployment to work with WaitForDeploymentsAvailable.
type deploymentsClientAdapter struct {
	client typedappsv1.DeploymentInterface
}

// Get fetches the deployment named by the key and updates the provided object.
func (c deploymentsClientAdapter) Get(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	deployment, err := c.client.Get(ctx, key.Name, metav1.GetOptions{})
	if deployObj, ok := obj.(*appsv1.Deployment); ok {
		deployment.DeepCopyInto(deployObj)
	}
	return err
}

// WaitForDeploymentsAvailableInput is the input for WaitForDeploymentsAvailable.
type WaitForDeploymentsAvailableInput struct {
	Getter     framework.Getter
	Deployment *appsv1.Deployment
	Clientset  *kubernetes.Clientset
}

// WaitForDeploymentsAvailable waits until the Deployment has status.Available = True, that signals that
// all the desired replicas are in place.
// This can be used to check if Cluster API controllers installed in the management cluster are working.
func WaitForDeploymentsAvailable(ctx context.Context, input WaitForDeploymentsAvailableInput, intervals ...interface{}) {
	start := time.Now()
	namespace, name := input.Deployment.GetNamespace(), input.Deployment.GetName()
	Byf("waiting for deployment %s/%s to be available", namespace, name)
	Log("starting to wait for deployment to become available")
	Eventually(func() bool {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		if err := input.Getter.Get(ctx, key, input.Deployment); err == nil {
			for _, c := range input.Deployment.Status.Conditions {
				if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
					return true
				}
			}
		}
		return false
	}, intervals...).Should(BeTrue(), func() string { return DescribeFailedDeployment(ctx, input) })
	Logf("Deployment %s/%s is now available, took %v", namespace, name, time.Since(start))
}

// GetWaitForDeploymentsAvailableInput is a convenience func to compose a WaitForDeploymentsAvailableInput
func GetWaitForDeploymentsAvailableInput(ctx context.Context, clusterProxy framework.ClusterProxy, name, namespace string, specName string) WaitForDeploymentsAvailableInput {
	Expect(clusterProxy).NotTo(BeNil())
	cl := clusterProxy.GetClient()
	var d = &appsv1.Deployment{}
	Eventually(func() error {
		return cl.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, d)
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())
	clientset := clusterProxy.GetClientSet()
	return WaitForDeploymentsAvailableInput{
		Deployment: d,
		Clientset:  clientset,
		Getter:     cl,
	}
}

// DescribeFailedDeployment returns detailed output to help debug a deployment failure in e2e.
func DescribeFailedDeployment(ctx context.Context, input WaitForDeploymentsAvailableInput) string {
	namespace, name := input.Deployment.GetNamespace(), input.Deployment.GetName()
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("Deployment %s/%s failed",
		namespace, name))
	b.WriteString(fmt.Sprintf("\nDeployment:\n%s\n", prettyPrint(input.Deployment)))
	b.WriteString(describeEvents(ctx, input.Clientset, namespace, name))
	return b.String()
}

// describeEvents returns a string summarizing recent events involving the named object(s).
func describeEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) string {
	b := strings.Builder{}
	if clientset == nil {
		b.WriteString("clientset is nil, so skipping output of relevant events")
	} else {
		opts := metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
			Limit:         20,
		}
		evts, err := clientset.CoreV1().Events(namespace).List(ctx, opts)
		if err != nil {
			b.WriteString(err.Error())
		} else {
			w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', tabwriter.FilterHTML)
			fmt.Fprintln(w, "LAST SEEN\tTYPE\tREASON\tOBJECT\tMESSAGE")
			for _, e := range evts.Items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s/%s\t%s\n", e.LastTimestamp, e.Type, e.Reason,
					strings.ToLower(e.InvolvedObject.Kind), e.InvolvedObject.Name, e.Message)
			}
			err := w.Flush()
			if err != nil {
				return ""
			}
		}
	}
	return b.String()
}

// prettyPrint returns a formatted JSON version of the object given.
func prettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// logCheckpoint prints a message indicating the start or end of the current test spec,
// including which Ginkgo node it's running on.
//
// Example output:
//
//	INFO: "With 1 worker node" started at Tue, 22 Sep 2020 13:19:08 PDT on Ginkgo node 2 of 3
//	INFO: "With 1 worker node" ran for 18m34s on Ginkgo node 2 of 3
func logCheckpoint(specTimes map[string]time.Time) {
	text := CurrentSpecReport().LeafNodeText
	start, started := specTimes[text]
	suiteConfig, reporterConfig := GinkgoConfiguration()
	if !started {
		start = time.Now()
		specTimes[text] = start
		fmt.Fprintf(GinkgoWriter, "INFO: \"%s\" started at %s on Ginkgo node %d of %d and junit test report to file %s\n", text,
			start.Format(time.RFC1123), GinkgoParallelProcess(), suiteConfig.ParallelTotal, reporterConfig.JUnitReport)
	} else {
		elapsed := time.Since(start)
		fmt.Fprintf(GinkgoWriter, "INFO: \"%s\" ran for %s on Ginkgo node %d of %d and reported junit test to file %s\n", text,
			elapsed.Round(time.Second), GinkgoParallelProcess(), suiteConfig.ParallelTotal, reporterConfig.JUnitReport)
	}
}

func getCdk8sAppProxy(ctx context.Context, c client.Client, clusterName string, cdk8sAppProxy addonsv1alpha1.Cdk8sAppProxy) (*addonsv1alpha1.Cdk8sAppProxy, error) {
	// Get the Cdk8sAppProxy using label selectors since we don't know the name of the Cdk8sAppProxy.
	result := &addonsv1alpha1.Cdk8sAppProxy{}
	if err := c.Get(ctx, client.ObjectKey{
		Name:      cdk8sAppProxy.Name,
		Namespace: cdk8sAppProxy.Namespace,
	}, result); err != nil {
		return nil, errors.Errorf("failed to get Cdk8sAppProxy %s/%s: %v", cdk8sAppProxy.Namespace, cdk8sAppProxy.Name, err)
	}

	return result, nil

	//labels := map[string]string{
	//	clusterv1.ClusterNameLabel: clusterName,
	//	// ToDo: addonsv1alpha1.Cdk8sAppProxyNameLabel: cdk8sAppProxy.Name,
	//}
	//if err := c.List(ctx, releaseList, client.InNamespace(cdk8sAppProxy.Namespace), client.MatchingLabels(labels)); err != nil {
	//	return nil, err
	//}
	//
	//if len(releaseList.Items) != 1 {
	//	return nil, errors.Errorf("expected 1 Cdk8sAppProxy, got %d", len(releaseList.Items))
	//}
	//
	//return &releaseList.Items[0], nil
}

// WaitForCdk8sAppProxyReadyInput is the input for WaitForCdk8sAppProxyReady.
type WaitForCdk8sAppProxyReadyInput struct {
	Getter           framework.Getter
	Cdk8sAppProxy    *addonsv1alpha1.Cdk8sAppProxy
	ExpectedRevision int
}

// ToDo: Before we can use this test, we need to make sure that the Cdk8sAppProxy controller sets
// the Ready condition to True when the Cdk8sAppProxy is ready, and that it updates the Revision field.
// WaitForCdk8sAppProxyReady waits until the Cdk8sAppProxy has ready condition = True, that signals that the cdk8s
// install was successful.
//func WaitForCdk8sAppProxyReady(ctx context.Context, input WaitForCdk8sAppProxyReadyInput, intervals ...interface{}) {
//	start := time.Now()
//	namespace, name := input.Cdk8sAppProxy.GetNamespace(), input.Cdk8sAppProxy.GetName()
//
//	Byf("waiting for Cdk8sAppProxy for %s/%s to be ready", input.Cdk8sAppProxy.GetNamespace(), input.Cdk8sAppProxy.GetName())
//	Log("starting to wait for Cdk8sAppProxy to become available")
//	cdk8sAppProxy := input.Cdk8sAppProxy
//	Eventually(func() bool {
//		key := client.ObjectKey{Namespace: namespace, Name: name}
//		if err := input.Getter.Get(ctx, key, cdk8sAppProxy); err == nil {
//			if conditions.IsTrue(cdk8sAppProxy, clusterv1.ReadyCondition) && cdk8sAppProxy.Status.Revision == input.ExpectedRevision {
//				return true
//			}
//		}
//		return false
//	}, intervals...).Should(BeTrue(), fmt.Sprintf("Cdk8sAppProxy %s/%s failed to become ready and have up to date revision: ready condition = %+v, revision = %v, expectedRevision = %v, full object is:\n%+v\n`", namespace, name, conditions.Get(input.Cdk8sAppProxy, clusterv1.ReadyCondition), cdk8sAppProxy.Status.Revision, input.ExpectedRevision, cdk8sAppProxy))
//	Logf("Cdk8sAppProxy %s/%s is now ready, took %v", namespace, name, time.Since(start))
//}

func WaitForCdk8sAppProxyReady(ctx context.Context, input WaitForCdk8sAppProxyReadyInput, intervals ...any) {
	start := time.Now()
	namespace, name := input.Cdk8sAppProxy.GetNamespace(), input.Cdk8sAppProxy.GetName()

	Byf("waiting for Cdk8sAppProxy %s/%s to be present in the cluster", namespace, name)
	Log("starting to wait for Cdk8sAppProxy to become available")
	cdk8sAppProxy := input.Cdk8sAppProxy
	Eventually(func() bool {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		if err := input.Getter.Get(ctx, key, cdk8sAppProxy); err == nil {
			return true
		}
		return false
	}, intervals...).Should(BeTrue(), fmt.Sprintf("Cdk8sAppProxy %s/%s not found in the cluster", namespace, name))
	Logf("Cdk8sAppProxy %s/%s is now present in the cluster, took %v", namespace, name, time.Since(start))
}

// GetWaitForCdk8sAppProxyReadyInput is a convenience func to compose a WaitForCdk8sAppProxyReadyInput.
func GetWaitForCdk8sAppProxyReadyInput(ctx context.Context, clusterProxy framework.ClusterProxy, clusterName string, cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy, expectedRevision int, specName string) WaitForCdk8sAppProxyReadyInput {
	Expect(clusterProxy).NotTo(BeNil())
	cl := clusterProxy.GetClient()

	Eventually(func() error {
		cdk8sapp, err := getCdk8sAppProxy(ctx, cl, clusterName, *cdk8sAppProxy)
		if err != nil {
			return err
		}

		annotations := cdk8sapp.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		//result, ok := annotations[addonsv1alpha1.IsReleaseNameGeneratedAnnotation]

		//isReleaseNameGenerated := ok && result == "true"
		// When an immutable field gets changed, the old Cdk8sAppProxy gets deleted and a new one comes online.
		// So we need to check to make sure the Cdk8sAppProxy we got is the right one by making sure the immutable fields match.
		switch {
		case cdk8sapp.Spec.GitRepository.URL != cdk8sAppProxy.Spec.GitRepository.URL:
			return errors.Errorf("RepoURL mismatch, got `%s` but Cdk8sAppProxy specifies `%s`", cdk8sapp.Spec.GitRepository.URL, cdk8sAppProxy.Spec.GitRepository.URL)
		}

		// If we made it past all the checks, then we have the correct Cdk8sAppProxy.
		cdk8sAppProxy = cdk8sapp

		return nil
	}, e2eConfig.GetIntervals(specName, "wait-cdk8sappproxy")...).Should(Succeed())
	return WaitForCdk8sAppProxyReadyInput{
		Cdk8sAppProxy:    cdk8sAppProxy,
		ExpectedRevision: expectedRevision,
		Getter:           cl,
	}
}

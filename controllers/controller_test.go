package controllers_test

import (
	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testRepository       = "github.com/eitco/test-repo"
	testNamespace1       = "test-namespace1"
	testNamespace2       = "test-namespace2"
	releaseFailedMessage = "unable to remove cdk8s release"
)

var (
	// failedCdk8sUninstall bool.

	newProxy = func(namespace string) *addonsv1alpha1.Cdk8sAppProxy {
		return &addonsv1alpha1.Cdk8sAppProxy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: addonsv1alpha1.GroupVersion.String(),
				Kind:       "Cdk8sAppProxy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cap",
				Namespace: namespace,
			},
			Spec: addonsv1alpha1.Cdk8sAppProxySpec{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"test-label": "test-value",
					},
				},
				GitRepository: &addonsv1alpha1.GitRepositorySpec{
					URL:       testRepository,
					Reference: "main",
					Path:      ".",
				},
			},
		}
	}

	newCluster = func(namespace string) *clusterv1.Cluster {
		return &clusterv1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "Cluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-1",
				Namespace: namespace,
				Labels: map[string]string{
					"test-label": "test-value",
				},
			},
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: clusterv1.ClusterNetwork{
					APIServerPort: int32(1234),
				},
			},
		}
	}

	// ToDo: Status Checks for deployed Cdk8sAppProxies
	// cdk8sAppProxyDeployed = &....
)

func newKubecnfigSecretForCluster(cluster *clusterv1.Cluster) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-kubeconfig",
			Namespace: cluster.Namespace,
		},
		StringData: map[string]string{
			secret.KubeconfigDataName: `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://127.0.0.1:8080
  name: ` + cluster.Name + `
contexts:
- context:
    cluster: ` + cluster.Name + `
  name: ` + cluster.Name + `
current-context: ` + cluster.Name + `
`,
		},
	}
}

var _ = Describe("Testing Cdk8sAppproxy Reconcile", func() {
	BeforeEach(func() {
		ns1 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace1,
			},
		}
		ns2 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace2,
			},
		}
		Expect(k8sClient.Create(ctx, ns1)).Should(Succeed())
		Expect(k8sClient.Create(ctx, ns2)).Should(Succeed())
	})

	AfterEach(func() {
		ns1 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace1}}
		ns2 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace2}}
		Expect(k8sClient.Delete(ctx, ns1)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, ns2)).Should(Succeed())
	})

	var (
		waitForCdk8sAppProxyCondition = func(objectKey client.ObjectKey, condition func(cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy) bool) {
			cdk8sapp := &addonsv1alpha1.Cdk8sAppProxy{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, objectKey, cdk8sapp); err != nil {
					return false
				}

				return condition != nil && condition(cdk8sapp)
			}, timeout, interval).Should(BeTrue())
		}

		install = func(cluster *clusterv1.Cluster, proxy *addonsv1alpha1.Cdk8sAppProxy) {
			err := k8sClient.Create(ctx, cluster)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Create(ctx, newKubecnfigSecretForCluster(cluster))
			Expect(err).ToNot(HaveOccurred())

			patch := client.MergeFrom(cluster.DeepCopy())
			// conditions.MarkTrue(cluster, clusterv1.ControlPlaneInitializedCondition)
			conditions.Set(cluster, metav1.Condition{
        Type: clusterv1.ClusterControlPlaneInitializedCondition,
				Status: metav1.ConditionTrue,
				Reason: "Successful",
			  Message: "Cdk8sAppProxy is ready",
			})
			err = k8sClient.Status().Patch(ctx, cluster, patch)
			Expect(err).ToNot(HaveOccurred())

			err = k8sClient.Create(ctx, proxy)
			Expect(err).ToNot(HaveOccurred())

			waitForCdk8sAppProxyCondition(client.ObjectKeyFromObject(proxy), func(cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy) bool {
				// ToDo: We return true, until Issue #13 has been resolved.
				// return conditions.IsTrue(cdk8sAppProxy, clusterv1.ReadyCondition)
				return true
			})
		}

		// deleteAndWaitCdk8sAppProxy = func(proxy *addonsv1alpha1.Cdk8sAppProxy) {
		// 	err := k8sClient.Delete(ctx, proxy)
		// 	Expect(err).ToNot(HaveOccurred())
		//
		// 	Eventually(func() bool {
		// 		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(proxy), &addonsv1alpha1.Cdk8sAppProxy{}); client.IgnoreNotFound(err) != nil {
		// 			return false
		// 		}
		//
		// 		return true
		// 	}, timeout, interval).Should(BeTrue())
		// }
	)

	// ToDo:
	It("Cdk8sAppProxy lifecycle happy path test", func() {
		cluster := newCluster(testNamespace1)
		cdk8sAppProxy := newProxy(testNamespace1)
		install(cluster, cdk8sAppProxy)

		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cdk8sAppProxy), cdk8sAppProxy)
		Expect(err).ToNot(HaveOccurred())
	})

	// It("Cdk8sAppProxy test with failed uninstall", func() {
	// 	cluster := newCluster(testNamespace2)
	// 	cdk8sAppProxy := newProxy(testNamespace2)
	// 	failedCdk8sUninstall = true
	// 	install(cluster, cdk8sAppProxy)
	//
	// 	err := k8sClient.Delete(ctx, cdk8sAppProxy)
	// 	Expect(err).ToNot(HaveOccurred())
	//
	// 	Consistently(func() bool {
	// 		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cdk8sAppProxy), cdk8sAppProxy); err != nil {
	// 			return false
	// 		}
	//
	// 		return true
	// 	}, timeout, interval).Should(BeTrue())
	//
	// 	readyCondition := conditions.Get(cdk8sAppProxy, clusterv1.ReadyCondition)
	// 	Expect(readyCondition).NotTo(BeNil())
	// 	Expect(readyCondition.Status).To(Equal(corev1.ConditionFalse))
	// 	Expect(readyCondition.Message).To(Equal(releaseFailedMessage))
	//
	// 	By("Making Cdk8sAppProxy uninstallable")
	// 	failedCdk8sUninstall = false
	// 	deleteAndWaitCdk8sAppProxy(cdk8sAppProxy)
	// })
})

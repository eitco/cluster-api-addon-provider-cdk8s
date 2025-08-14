package resourcer

import (
	"context"
	"strings"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resourcer interface {
	Apply(ctx context.Context, cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy, parsedResources []*unstructured.Unstructured, logger logr.Logger) (err error)
	Check(ctx context.Context, cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy, parsedResources []*unstructured.Unstructured, logger logr.Logger) (missingResources bool, err error)
}

type Implementer struct {
	client.Client
}

// Apply applies resources to the target clusters.
func (i *Implementer) Apply(ctx context.Context, cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy, parsedResources []*unstructured.Unstructured, logger logr.Logger) (err error) {
	clusters, err := i.clusterList(ctx, cdk8sAppProxy, logger)
	if err != nil {
		logger.Error(err, "failed to list clusters")

		return err
	}

	for _, cluster := range clusters.Items {
		c, err := i.clusterClient(ctx, cluster.Namespace, cluster.Name)
		if err != nil {
			logger.Error(err, "failed to get cluster client")

			break
		}

		for _, resource := range parsedResources {
			resources := resource.DeepCopy()
			gvr := resource.GroupVersionKind().GroupVersion().WithResource(getPluralFromKind(resource.GetKind()))
			applyOpts := metav1.ApplyOptions{FieldManager: "cdk8sappproxy-controller", Force: true}

			_, err = c.Resource(gvr).Namespace(resources.GetNamespace()).Apply(ctx, resources.GetName(), resources, applyOpts)
			if err != nil {
				logger.Error(err, "failed to apply resource")
				conditions.MarkFalse(cdk8sAppProxy, addonsv1alpha1.DeploymentProgressingCondition, addonsv1alpha1.ResourceApplyFailedReason, clusterv1.ConditionSeverityError, "Failed to apply resource to target clusters.")

				break
			}
		}

		cdk8sAppProxy.Status.ObservedGeneration = cdk8sAppProxy.Generation
		conditions.MarkTrue(cdk8sAppProxy, addonsv1alpha1.DeploymentProgressingCondition)
		if err = i.Status().Update(ctx, cdk8sAppProxy); err != nil {
			logger.Error(err, "failed to update cdk8sAppProxy status")

			return err
		}
	}

	return err
}

// Check checks if the provided resource exists on the target cluster.
func (i *Implementer) Check(ctx context.Context, cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy, parsedResources []*unstructured.Unstructured, logger logr.Logger) (missingResources bool, err error) {
	missingResources = false

	clusters, err := i.clusterList(ctx, cdk8sAppProxy, logger)
	if err != nil {
		logger.Error(err, "failed to list clusters")

		return missingResources, err
	}

	for _, cluster := range clusters.Items {
		c, err := i.clusterClient(ctx, cluster.Namespace, cluster.Name)
		if err != nil {
			logger.Error(err, "failed to get cluster client")

			return missingResources, err
		}

		for _, resource := range parsedResources {
			gvr := resource.GroupVersionKind().GroupVersion().WithResource(getPluralFromKind(resource.GetKind()))
			resourceGetter := c.Resource(gvr)
			ns := resource.GetNamespace()

			if ns != "" {
				_, err = resourceGetter.Namespace(ns).Get(ctx, resource.GetName(), metav1.GetOptions{})
			} else {
				_, err = resourceGetter.Get(ctx, resource.GetName(), metav1.GetOptions{})
			}
			if err != nil {
				if apierrors.IsNotFound(err) {
					missingResources = true

					continue
				}
				logger.Error(err, "failed to check if resource exists")

				return missingResources, err
			}
		}
	}

	return missingResources, err
}

func (i *Implementer) clusterList(ctx context.Context, cdk8sAppProxy *addonsv1alpha1.Cdk8sAppProxy, logger logr.Logger) (clusterList clusterv1.ClusterList, err error) {
	selector, err := metav1.LabelSelectorAsSelector(&cdk8sAppProxy.Spec.ClusterSelector)
	if err != nil {
		logger.Error(err, "failed to convert label selector to selector")

		return clusterList, err
	}

	if err := i.List(ctx, &clusterList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		logger.Error(err, "failed to list clusters")

		return clusterList, err
	}

	return clusterList, err
}

func (i *Implementer) clusterClient(ctx context.Context, secretNamespace, clusterName string) (dynamicClient dynamic.Interface, err error) {
	kubeconfigSecretName := clusterName + "-kubeconfig"
	kubeconfigSecret := &corev1.Secret{}
	if err = i.Get(ctx, client.ObjectKey{Namespace: secretNamespace, Name: kubeconfigSecretName}, kubeconfigSecret); err != nil {
		return dynamicClient, err
	}

	kubeconfigData, ok := kubeconfigSecret.Data["value"]
	if !ok || len(kubeconfigData) == 0 {
		return dynamicClient, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return dynamicClient, err
	}

	dynamicClient, err = dynamic.NewForConfig(restConfig)
	if err != nil {
		return dynamicClient, err
	}

	return dynamicClient, err
}

// TODO: This is a naive pluralization and might not work for all kinds.
// A more robust solution would use discovery client or a predefined map.
func getPluralFromKind(kind string) string {
	lowerKind := strings.ToLower(kind)
	if strings.HasSuffix(lowerKind, "s") {
		return lowerKind + "es"
	}
	if strings.HasSuffix(lowerKind, "y") {
		return strings.TrimSuffix(lowerKind, "y") + "ies"
	}

	return lowerKind + "s"
}

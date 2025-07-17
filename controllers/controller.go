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

package controllers

import (
	"context"
	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	Finalizer = "cdk8sappproxy.addons.cluster.x-k8s.io/finalizer"
)

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1alpha1.Cdk8sAppProxy{}).
		Watches(&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToCdk8sAppProxyMapper)).
		Complete(r)
}

// ClusterToCdk8sAppProxyMapper is a handler.ToRequestsFunc to be used to enqeue requests for Cdk8sAppProxyReconciler.
// It maps CAPI Cluster events to Cdk8sAppProxy events.
func (r *Reconciler) ClusterToCdk8sAppProxyMapper(ctx context.Context, o client.Object) (requests []ctrl.Request) {
	logger := log.FromContext(ctx)
	cluster, ok := o.(*clusterv1.Cluster)
	if !ok {
		logger.Error(errors.Errorf("unexpected type %T, expected Cluster", o), "failed to cast object to Cluster", "object", o)

		return requests
	}

	logger = logger.WithValues("clusterName", cluster.Name, "clusterNamespace", cluster.Namespace)
	logger.Info("ClusterToCdk8sAppProxyMapper triggered for cluster")

	proxies := &addonsv1alpha1.Cdk8sAppProxyList{}
	// List all Cdk8sAppProxies in the same namespace as the Cluster.
	// Adjust if Cdk8sAppProxy can be in a different namespace or cluster-scoped.
	// For now, assuming Cdk8sAppProxy is namespace-scoped and in the same namespace as the triggering Cluster's Cdk8sAppProxy object (which is usually the management cluster's default namespace).
	// However, Cdk8sAppProxy resources themselves select clusters across namespaces.
	// So, we should list Cdk8sAppProxies from all namespaces if the controller has cluster-wide watch permissions for them.
	// If the controller is namespace-scoped for Cdk8sAppProxy, this list will be limited.
	// For this example, let's assume a cluster-wide list for Cdk8sAppProxy.
	if err := r.List(ctx, proxies); err != nil { // staticcheck: QF1008
		logger.Error(err, "failed to list Cdk8sAppProxies")

		return requests
	}
	logger.Info("Checking Cdk8sAppProxies for matches", "count", len(proxies.Items))

	for _, proxy := range proxies.Items {
		proxyLogger := logger.WithValues("cdk8sAppProxyName", proxy.Name, "cdk8sAppProxyNamespace", proxy.Namespace)
		proxyLogger.Info("Evaluating Cdk8sAppProxy")

		selector, err := metav1.LabelSelectorAsSelector(&proxy.Spec.ClusterSelector)
		if err != nil {
			proxyLogger.Error(err, "failed to parse ClusterSelector for Cdk8sAppProxy")

			continue
		}
		proxyLogger.Info("Parsed ClusterSelector", "selector", selector.String())

		if selector.Matches(labels.Set(cluster.GetLabels())) {
			proxyLogger.Info("Cluster labels match Cdk8sAppProxy selector, enqueuing request")
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: proxy.Namespace,
					Name:      proxy.Name,
				},
			})
		} else {
			proxyLogger.Info("Cluster labels do not match Cdk8sAppProxy selector")
		}
	}

	logger.Info("ClusterToCdk8sAppProxyMapper finished", "requestsEnqueued", len(requests))

	return requests
}

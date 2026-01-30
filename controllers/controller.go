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
	"os"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	gitoperator "github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/git"
	"github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/resourcer"
	"github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/synthesizer"
	"github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/utils"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&addonsv1alpha1.Cdk8sAppProxy{}).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToCdk8sAppProxyMapper),
		).
		Complete(r)
}

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=cdk8sappproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=cdk8sappproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=cdk8sappproxies/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (controller ctrl.Result, err error) {
	logs := ctrl.LoggerFrom(ctx).WithValues("cdk8sappproxy", req.NamespacedName)
	logs.Info("Reconciling CDk8sAppProxy")

	synthImpl := &synthesizer.Implementer{}
	resourcerImpl := &resourcer.Implementer{
		Client: r.Client,
	}
	cdk8sAppProxy := &addonsv1alpha1.Cdk8sAppProxy{}

	if err = r.Get(ctx, req.NamespacedName, cdk8sAppProxy); err != nil {
		if apierrors.IsNotFound(err) {
			logs.Error(err, "cdk8sAppProxy resource not found")

			return ctrl.Result{}, err
		}
		logs.Error(err, "Failed to get cdk8sAppProxy")

		return ctrl.Result{}, err
	}

	repoURL := cdk8sAppProxy.Spec.GitRepository.URL
	branch := cdk8sAppProxy.Spec.GitRepository.Reference
	directory := "/tmp/cdk8s-" + cdk8sAppProxy.Namespace + "-" + cdk8sAppProxy.Name + "-" + branch

	defer func(path string) {
		err = os.RemoveAll(path)
		if err != nil {
			logs.Error(err, "Failed to clean-up directory", "path", path)
		}
	}(directory)

	// Fetch secret for Git authentication if provided.
	secretRef, err := utils.FetchSecret(ctx, r.Client, cdk8sAppProxy.Namespace, cdk8sAppProxy.Spec.GitRepository, logs)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check access before Cloning
	gitImpl := &gitoperator.Implementer{}
	accessible, requiredAuth, err := gitImpl.CheckAccess(repoURL, secretRef, logs)
	if err != nil {
		logs.Error(err, "Failed to check repository access")

		return ctrl.Result{}, err
	}

	if requiredAuth && len(secretRef) == 0 {
		logs.Error(err, "Repository requires authentication but no secretRef was provided.")

		return ctrl.Result{}, err
	}

	if !accessible {
		logs.Error(err, "repository is not accessible. Access Denied")

		return ctrl.Result{}, err
	}

	if !requiredAuth {
		secretRef = nil
	}

	err = gitImpl.Clone(repoURL, secretRef, branch, directory, logs)
	if err != nil {
		conditions.Set(cdk8sAppProxy, metav1.Condition{
			Type:    clusterv1.AvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "Failed",
			Message: "Failed to clone Git Repository",
		})

		return ctrl.Result{}, err
	}

	logs.Info("Starting to synthesize resources", "directory", directory)
	parsedResources, err := synthImpl.Synthesize(directory, cdk8sAppProxy, logs, ctx)
	if err != nil {
		logs.Error(err, "failed to synthesize resources")
		conditions.Set(cdk8sAppProxy, metav1.Condition{
			Type:    clusterv1.AvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "Failed",
			Message: "Failed to synth cdk8s code",
		})

		return ctrl.Result{}, err
	}
	logs.Info("Synthesized resources", "count", len(parsedResources))

	err = resourcerImpl.Apply(ctx, cdk8sAppProxy, parsedResources, logs)
	if err != nil {
		logs.Error(err, "failed to apply resources")
		conditions.Set(cdk8sAppProxy, metav1.Condition{
			Type:    clusterv1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "Failed",
			Message: "Failed to apply resources",
		})

		return ctrl.Result{}, err
	}

	missingResource, err := resourcerImpl.Check(ctx, cdk8sAppProxy, parsedResources, logs)
	if err != nil {
		logs.Error(err, "failed to check for resource existence")

		return ctrl.Result{}, err
	}

	if !missingResource {
		conditions.Set(cdk8sAppProxy, metav1.Condition{
			Type:    clusterv1.ReadyCondition,
			Status:  metav1.ConditionTrue,
			Reason:  "Successful",
			Message: "Cdk8sAppProxy is ready",
		})
	}

	if err = r.Status().Update(ctx, cdk8sAppProxy); err != nil {
		logs.Error(err, "failed to update cdk8sAppProxy status")

		return ctrl.Result{}, err
	}

	logs.Info("Reconciliation finished successfully")

	return ctrl.Result{}, err
}

// ClusterToCdk8sAppProxyMapper is a handler.ToRequestsFunc to be used to enqeue requests for Cdk8sAppProxyReconciler.
// It maps CAPI Cluster events to Cdk8sAppProxy events.
func (r *Reconciler) ClusterToCdk8sAppProxyMapper(ctx context.Context, o client.Object) (results []ctrl.Request) {
	logs := ctrl.LoggerFrom(ctx)

	cluster, ok := o.(*clusterv1.Cluster)
	if !ok {
		logs.Error(errors.Errorf("expected a Cluster but got %T", o), "failed to map object to Cdk8sAppProxy")

		return results
	}

	cdk8sappproxies := &addonsv1alpha1.Cdk8sAppProxyList{}

	if err := r.List(ctx, cdk8sappproxies, client.InNamespace(cluster.Namespace)); err != nil {
		return results
	}

	for _, cdk8sAppProxy := range cdk8sappproxies.Items {
		selector, err := metav1.LabelSelectorAsSelector(&cdk8sAppProxy.Spec.ClusterSelector)
		if err != nil {
			logs.Error(err, "failed to parse ClusterSelector for Cdk8sAppProxy", "cdk8sAppProxy", cdk8sAppProxy.Name)

			return results
		}

		if selector.Matches(labels.Set(cluster.Labels)) {
			results = append(results, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: cdk8sAppProxy.Namespace,
					Name:      cdk8sAppProxy.Name,
				},
			})
		}
	}

	return results
}

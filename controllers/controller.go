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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	Finalizer = "cdk8sappproxy.addons.cluster.x-k8s.io/finalizer"
)

type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
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
	// logs := ctrl.LoggerFrom(ctx).WithValues("cdk8sappproxy", req.NamespacedName)
	logger := log.FromContext(ctx).WithValues("cdk8sappproxy", req.NamespacedName)
	logger.Info("Starting Reconciler")

	gitImpl := &gitoperator.GitImplementer{}
	synthImpl := &synthesizer.Implementer{}
	resourcerImpl := &resourcer.Implementer{
		Client: r.Client,
	}
	cdk8sAppProxy := &addonsv1alpha1.Cdk8sAppProxy{}

	if err := r.Get(ctx, req.NamespacedName, cdk8sAppProxy); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "cdk8sAppProxy resource not found")

			return ctrl.Result{}, err
		}

		logger.Error(err, "Failed to get cdk8sAppProxy")

		return ctrl.Result{}, err
	}

	repoURL := cdk8sAppProxy.Spec.GitRepository.URL
	branch := cdk8sAppProxy.Spec.GitRepository.Reference
	directory := "/tmp/cdk8s-" + cdk8sAppProxy.Namespace + "-" + cdk8sAppProxy.Name + "-" + branch
	secretKey := cdk8sAppProxy.Spec.GitRepository.SecretKey
	secretName := cdk8sAppProxy.Spec.GitRepository.SecretRef

	defer os.RemoveAll(directory)

	var (
		secretRef []byte
		ok        bool
	)

	if secretName != "" {
		namespacedSecretKey := types.NamespacedName{
			Namespace: cdk8sAppProxy.Namespace,
			Name:      secretName,
		}

		secret := &corev1.Secret{}
		if err = r.Get(ctx, namespacedSecretKey, secret); err != nil {
			logger.Error(err, "Error getting the Secret specified in SecretRef")

			return ctrl.Result{}, err
		}

		secretRef, ok = secret.Data[secretKey]
		if !ok {
			logger.Error(err, "secret '%s' does not contain data key '%s'", secretName, secretKey)

			return ctrl.Result{}, err
		}
	}

	// Check access before (interface)Cloning
	_, requiredAuth, err := gitImpl.CheckAccess(repoURL, secretRef, logger)
	if err != nil {
		logger.Error(err, "Failed to check repository access")

		return ctrl.Result{}, err
	}

	if requiredAuth && len(secretRef) == 0 {
		logger.Error(err, "Repository requires authentication but no secretRef was provided.")

		return ctrl.Result{}, err
	}

	if !requiredAuth {
		secretRef = nil
	}

	err = gitImpl.Clone(repoURL, secretRef, branch, directory, logger)
	if err != nil {
		logger.Error(err, "Failed to clone git repository", "repoURL", repoURL, "directory", directory)
		conditions.Set(cdk8sAppProxy, metav1.Condition{
			Type:    clusterv1.AvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "Failed",
			Message: "Failed to clone Git Repository",
		})

		return ctrl.Result{}, err
	}

	parsedResources, err := synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
	if err != nil {
		logger.Error(err, "failed to synthesize resources")
		conditions.Set(cdk8sAppProxy, metav1.Condition{
			Type:    clusterv1.AvailableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "Failed",
			Message: "Failed to synth cdk8s code",
		})

		return ctrl.Result{}, err
	}

	err = resourcerImpl.Apply(ctx, cdk8sAppProxy, parsedResources, logger)
	if err != nil {
		logger.Error(err, "failed to apply resources")
		conditions.Set(cdk8sAppProxy, metav1.Condition{
			Type:    clusterv1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "Failed",
			Message: "Failed to apply resources",
		})

		return ctrl.Result{}, err
	}

	missingResource, err := resourcerImpl.Check(ctx, cdk8sAppProxy, parsedResources, logger)
	if err != nil {
		logger.Error(err, "failed to check for resource existence")

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
		logger.Error(err, "failed to update cdk8sAppProxy status")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}

// ClusterToCdk8sAppProxyMapper is a handler.ToRequestsFunc to be used to enqeue requests for Cdk8sAppProxyReconciler.
// It maps CAPI Cluster events to Cdk8sAppProxy events.
func (r *Reconciler) ClusterToCdk8sAppProxyMapper(ctx context.Context, o client.Object) (results []ctrl.Request) {
	log := ctrl.LoggerFrom(ctx)

	cluster, ok := o.(*clusterv1.Cluster)
	if !ok {
		log.Error(errors.Errorf("expected a Cluster but got %T", o), "failed to map object to Cdk8sAppProxy")

		return results
	}

	cdk8sappproxies := &addonsv1alpha1.Cdk8sAppProxyList{}

	if err := r.List(ctx, cdk8sappproxies, client.InNamespace(cluster.Namespace)); err != nil {
		return results
	}

	for _, cdk8sAppProxy := range cdk8sappproxies.Items {
		selector, err := metav1.LabelSelectorAsSelector(&cdk8sAppProxy.Spec.ClusterSelector)
		if err != nil {
			log.Error(err, "failed to parse ClusterSelector for Cdk8sAppProxy", "cdk8sAppProxy", cdk8sAppProxy.Name)

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

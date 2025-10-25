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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gitoperator "github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/git"
	"github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/resourcer"
	"github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/synthesizer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api/util/conditions"
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
	directory := "/tmp/cdk8s-"+cdk8sAppProxy.Namespace+"-"+cdk8sAppProxy.Name+"-"+branch
	secretKey := cdk8sAppProxy.Spec.GitRepository.SecretKey
	secretName := cdk8sAppProxy.Spec.GitRepository.SecretRef
	var secretRef []byte
	var ok bool

	if secretName != "" {
		logger.Info("SecretRef provided, fetching secret from cluster", "secretName", secretName)

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

	logger.Info("Checking if directory already exists")
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		// Proceed with cloning knowing wether to use authentication
		// Controlflow first tries no authentication and then provides a secretRef
		if !requiredAuth {
			secretRef = nil
		}

		logger.Info("Cloning Repo")
		err = gitImpl.Clone(repoURL, secretRef, directory, logger)
		if err != nil {
			logger.Error(err, "Failed to clone git repository", "repoURL", repoURL, "directory", directory)
			conditions.MarkFalse(cdk8sAppProxy, addonsv1alpha1.GitCloneCondition, addonsv1alpha1.GitCloneFailedReason, clusterv1.ConditionSeverityError, "Failed to clone the git repository") 

			return ctrl.Result{}, err
		}

		logger.Info("Parsing resources and synthing")
		parsedResources, err := synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
		if err != nil {
			logger.Error(err, "failed to synthesize resources")
			conditions.MarkFalse(cdk8sAppProxy, addonsv1alpha1.SynthCondition, addonsv1alpha1.SynthFailedReason, clusterv1.ConditionSeverityError, "Failed to synth cdk8s code")

			return ctrl.Result{}, err
		}

		logger.Info("Applying Synthed Code")
		err = resourcerImpl.Apply(ctx, cdk8sAppProxy, parsedResources, logger)
		if err != nil {
			logger.Error(err, "failed to apply resources")
		  conditions.MarkFalse(cdk8sAppProxy, addonsv1alpha1.ApplyResourcesCondition, addonsv1alpha1.ApplyResourcesFailedReason, clusterv1.ConditionSeverityError, "Failed to apply cdk8s resources to the target cluster") 	

			return ctrl.Result{}, err
		}
		logger.Info("Successfully applied resources")

		conditions.MarkTrue(cdk8sAppProxy, clusterv1.ReadyCondition)
		if err = r.Status().Update(ctx, cdk8sAppProxy); err != nil {
			logger.Error(err, "failed to update cdk8sAppProxy status")

			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, err
}

// ClusterToCdk8sAppProxyMapper is a handler.ToRequestsFunc to be used to enqeue requests for Cdk8sAppProxyReconciler.
// It maps CAPI Cluster events to Cdk8sAppProxy events.
func (r *Reconciler) ClusterToCdk8sAppProxyMapper(ctx context.Context, o client.Object) (results []ctrl.Request) {
	logger := log.FromContext(ctx)

	cluster, ok := o.(*clusterv1.Cluster)
	if !ok {
		logger.Info("Failed to get clusters")

		return results
	}

	logger = log.FromContext(ctx).WithValues("clusterName", cluster.Name, "clusterNamespace", cluster.Namespace)
	
	proxies := &addonsv1alpha1.Cdk8sAppProxyList{}

	if err := r.List(ctx, proxies); err != nil {
		logger.Error(err, "failed to list Cdk8sAppProxies")

		return results
	}

	for _, proxy := range proxies.Items {
		selector, err := metav1.LabelSelectorAsSelector(&proxy.Spec.ClusterSelector)
		if err != nil {
			logger.Error(err, "failed to parse ClusterSelector for Cdk8sAppProxy")

			continue
		}

		if selector.Matches(labels.Set(cluster.GetLabels())) {
			results = append(results, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: proxy.Namespace,
					Name:      proxy.Name,
				},
			})
		} else {
			logger.Info("Cluster labels do not match Cdk8sAppProxy selector")
		}
	}

	return results
}

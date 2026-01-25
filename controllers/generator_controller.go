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
	"fmt"
	"time"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	gitoperator "github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/git"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GeneratorReconciler reconciles a Cdk8sAppProxyGenerator object.
type GeneratorReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ProviderClient gitoperator.ProviderClient
}

// SetupWithManager sets up the controller with the Manager.
func (r *GeneratorReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&addonsv1alpha1.Cdk8sAppProxyGenerator{}).
		Owns(&addonsv1alpha1.Cdk8sAppProxy{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=cdk8sappproxygenerators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=cdk8sappproxygenerators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=cdk8sappproxygenerators/finalizers,verbs=update
//+kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=cdk8sappproxies,verbs=get;list;watch;create;update;patch;delete

func (r *GeneratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("cdk8sappproxygenerator", req.NamespacedName)
	logger.Info("Starting Generator Reconciler")

	generator := &addonsv1alpha1.Cdk8sAppProxyGenerator{}
	if err := r.Get(ctx, req.NamespacedName, generator); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Determine poll interval.
	pollInterval := 5 * time.Minute
	if generator.Spec.PollInterval != nil {
		pollInterval = generator.Spec.PollInterval.Duration
	}

	// Check if it's time to poll.
	if generator.Status.LastPolledTime != nil {
		nextPoll := generator.Status.LastPolledTime.Add(pollInterval)
		if time.Now().Before(nextPoll) {
			return ctrl.Result{RequeueAfter: time.Until(nextPoll)}, nil
		}
	}

	// Fetch secret for Git authentication if provided.
	var secretRef []byte
	if generator.Spec.Source.SecretRef != "" {
		secret := &corev1.Secret{}
		secretKey := types.NamespacedName{
			Namespace: generator.Namespace,
			Name:      generator.Spec.Source.SecretRef,
		}
		if err := r.Get(ctx, secretKey, secret); err != nil {
			logger.Error(err, "failed to get secret", "secret", generator.Spec.Source.SecretRef)

			return ctrl.Result{}, err
		}
		var ok bool
		secretRef, ok = secret.Data[generator.Spec.Source.SecretKey]
		if !ok {
			err := fmt.Errorf("secret %s does not contain key %s", generator.Spec.Source.SecretRef, generator.Spec.Source.SecretKey)
			logger.Error(err, "secret key not found")

			return ctrl.Result{}, err
		}
	}

	// List pull requests.
	prs, err := r.ProviderClient.ListPullRequests(ctx, generator.Spec.Source.URL, secretRef, logger)
	if err != nil {
		logger.Error(err, "failed to list pull requests")

		return ctrl.Result{}, err
	}

	// Process each PR.
	for _, pr := range prs {
		if err := r.reconcilePR(ctx, generator, pr); err != nil {
			logger.Error(err, "failed to reconcile PR", "prNumber", pr.Number)
			// Continue with other PRs.
		}
	}

	// Update last polled time.
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &addonsv1alpha1.Cdk8sAppProxyGenerator{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return err
		}
		latest.Status.LastPolledTime = &metav1.Time{Time: time.Now()}

		return r.Status().Update(ctx, latest)
	})
	if err != nil {
		logger.Error(err, "failed to update status")

		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: pollInterval}, nil
}

func (r *GeneratorReconciler) reconcilePR(ctx context.Context, generator *addonsv1alpha1.Cdk8sAppProxyGenerator, pr gitoperator.PullRequest) error {
	logger := log.FromContext(ctx).WithValues("prNumber", pr.Number)

	// Apply filters.
	if len(generator.Spec.Filters) > 0 {
		match := false
		for _, filter := range generator.Spec.Filters {
			if filter.BranchMatch == "" || filter.BranchMatch == pr.BaseBranch {
				match = true

				break
			}
		}
		if !match {
			logger.Info("PR does not match any filters, skipping", "baseBranch", pr.BaseBranch)

			return nil
		}
	}

	// Define the Cdk8sAppProxy name.
	proxyName := fmt.Sprintf("%s-pr-%d", generator.Name, pr.Number)

	proxy := &addonsv1alpha1.Cdk8sAppProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        proxyName,
			Namespace:   generator.Namespace,
			Labels:      generator.Spec.Template.Metadata.Labels,
			Annotations: generator.Spec.Template.Metadata.Annotations,
		},
	}

	if proxy.Labels == nil {
		proxy.Labels = make(map[string]string)
	}
	proxy.Labels["addons.cluster.x-k8s.io/generator-name"] = generator.Name
	proxy.Labels["addons.cluster.x-k8s.io/pr-number"] = fmt.Sprintf("%d", pr.Number)

	// Set OwnerReference.
	if err := ctrl.SetControllerReference(generator, proxy, r.Scheme); err != nil {
		return errors.Wrap(err, "failed to set controller reference")
	}

	// Render the template spec.
	proxy.Spec = generator.Spec.Template.Spec
	// Override GitRepository information with PR specifics.
	if proxy.Spec.GitRepository == nil {
		proxy.Spec.GitRepository = &addonsv1alpha1.GitRepositorySpec{}
	}
	proxy.Spec.GitRepository.URL = generator.Spec.Source.URL
	proxy.Spec.GitRepository.Reference = pr.Branch
	proxy.Spec.GitRepository.SecretRef = generator.Spec.Source.SecretRef
	proxy.Spec.GitRepository.SecretKey = generator.Spec.Source.SecretKey
	if generator.Spec.Source.Path != "" {
		proxy.Spec.GitRepository.Path = generator.Spec.Source.Path
	}
	if generator.Spec.Path != "" {
		proxy.Spec.GitRepository.Path = generator.Spec.Path
	}

	// Create or Update the Cdk8sAppProxy.
	existingProxy := &addonsv1alpha1.Cdk8sAppProxy{}
	err := r.Get(ctx, types.NamespacedName{Namespace: proxy.Namespace, Name: proxy.Name}, existingProxy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating Cdk8sAppProxy for PR", "proxyName", proxyName)

			return r.Create(ctx, proxy)
		}

		return err
	}

	logger.Info("Updating Cdk8sAppProxy for PR", "proxyName", proxyName, "ref", proxy.Spec.GitRepository.Reference, "path", proxy.Spec.GitRepository.Path)
	existingProxy.Spec = proxy.Spec
	existingProxy.Labels = proxy.Labels
	existingProxy.Annotations = proxy.Annotations

	return r.Update(ctx, existingProxy)
}

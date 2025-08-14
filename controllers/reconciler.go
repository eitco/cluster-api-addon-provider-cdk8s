package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	gitoperator "github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/git"
	"github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/resourcer"
	"github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/synthesizer"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
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
	var pollInterval time.Duration

	if err := r.Get(ctx, req.NamespacedName, cdk8sAppProxy); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Cdk8sAppProxy resource not found.")
			controller = ctrl.Result{RequeueAfter: 5 * time.Minute}

			return controller, err
		}
		logger.Error(err, "Failed to get Cdk8sAppProxy")
		controller = ctrl.Result{RequeueAfter: 5 * time.Minute}

		return controller, errors.Wrapf(err, "failed to get Cdk8sAppProxy %s/%s", req.Namespace, req.Name)
	}

	if cdk8sAppProxy.Spec.GitRepository.ReferencePollInterval == nil {
		pollInterval = 5 * time.Minute
	} else {
		pollInterval = cdk8sAppProxy.Spec.GitRepository.ReferencePollInterval.Duration
	}

	repoUrl := cdk8sAppProxy.Spec.GitRepository.URL
	branch := cdk8sAppProxy.Spec.GitRepository.Reference
	directory := fmt.Sprintf("/tmp/cdk8s-%s-%s-%s", cdk8sAppProxy.Namespace, cdk8sAppProxy.Name, branch)

	if cdk8sAppProxy.Spec.GitRepository.URL == "" {
		logger.Info("GitRepository URL is not specified in Cdk8sAppProxy spec", "cdk8sAppProxy", cdk8sAppProxy.Name)
		controller = ctrl.Result{RequeueAfter: pollInterval}

		return controller, errors.New("GitRepository URL is not specified in Cdk8sAppProxy spec")
	}

	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err = gitImpl.Clone(repoUrl, directory, logger)
		if err != nil {
			logger.Error(err, "Failed to clone git repository", "repoUrl", repoUrl, "directory", directory)
			controller = ctrl.Result{RequeueAfter: pollInterval}

			return controller, err
		}

		parsedResources, err := synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
		if err != nil {
			logger.Error(err, "failed to synthesize resources")
			controller = ctrl.Result{RequeueAfter: pollInterval}

			return controller, err
		}

		err = resourcerImpl.Apply(ctx, cdk8sAppProxy, parsedResources, logger)
		if err != nil {
			logger.Error(err, "failed to apply resources")
			controller = ctrl.Result{RequeueAfter: pollInterval}

			return controller, err
		}

		// ToDo: https://github.com/eitco/cluster-api-addon-provider-cdk8s/issues/13
		// Set Condition to ready
		// conditions.MarkTrue(cdk8sAppProxy, addonsv1alpha1.Cdk8sAppProxyReadyCondition)

		// Set the revision in the Cdk8sAppProxy status
		// cdk8sAppProxy.Status.Revision = 1
	}

	parsedResources, err := synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
	if err != nil {
		logger.Error(err, "failed to synthesize resources")
		controller = ctrl.Result{RequeueAfter: pollInterval}

		return controller, err
	}

	missingResources, err := resourcerImpl.Check(ctx, cdk8sAppProxy, parsedResources, logger)
	if err != nil {
		logger.Error(err, "failed to check resources")
		controller = ctrl.Result{RequeueAfter: pollInterval}

		return controller, err
	}

	// ToDo: Maybe we need to loop for each resource
	if missingResources {
		logger.Info("Missing resources detected, proceeding with reconciliation.")
		parsedResources, err = synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
		if err != nil {
			logger.Error(err, "failed to synthesize resources")
			controller = ctrl.Result{RequeueAfter: pollInterval}

			return controller, err
		}

		err = resourcerImpl.Apply(ctx, cdk8sAppProxy, parsedResources, logger)
		if err != nil {
			logger.Error(err, "failed to apply resources")
			controller = ctrl.Result{RequeueAfter: pollInterval}

			return controller, err
		}

		// ToDo: https://github.com/eitco/cluster-api-addon-provider-cdk8s/issues/13
		// Set Condition to ready
		// conditions.MarkTrue(cdk8sAppProxy, addonsv1alpha1.Cdk8sAppProxyReadyCondition)

		// Set the revision in the Cdk8sAppProxy status
		// cdk8sAppProxy.Status.Revision++

		controller = ctrl.Result{RequeueAfter: pollInterval}

		return controller, err
	}

	hashChanges, err := gitImpl.Poll(repoUrl, branch, directory, logger)
	if err != nil {
		logger.Error(err, "Failed to poll git repository", "repoUrl", repoUrl, "branch", branch)
		controller = ctrl.Result{RequeueAfter: pollInterval}

		return controller, err
	}

	if hashChanges {
		logger.Info("Detected changes in git repository, proceeding with reconciliation.")

		if err := os.RemoveAll(directory); err != nil {
			logger.Error(err, "Failed to clean up directory", "directory", directory)
			controller = ctrl.Result{RequeueAfter: pollInterval}

			return controller, err
		}

		err = gitImpl.Clone(cdk8sAppProxy.Spec.GitRepository.URL, directory, logger)
		if err != nil {
			logger.Error(err, "failed to clone git repository")
			controller = ctrl.Result{RequeueAfter: pollInterval}

			return controller, err
		}

		parsedResources, err = synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
		if err != nil {
			logger.Error(err, "failed to synthesize resources")
			controller = ctrl.Result{RequeueAfter: pollInterval}

			return controller, err
		}

		err = resourcerImpl.Apply(ctx, cdk8sAppProxy, parsedResources, logger)
		if err != nil {
			logger.Error(err, "failed to apply resources")
			controller = ctrl.Result{RequeueAfter: pollInterval}

			return controller, err
		}

		// ToDo: https://github.com/eitco/cluster-api-addon-provider-cdk8s/issues/13
		// Set Condition to ready
		// conditions.MarkTrue(cdk8sAppProxy, addonsv1alpha1.Cdk8sAppProxyReadyCondition)

		// Set the revision in the Cdk8sAppProxy status
		// cdk8sAppProxy.Status.Revision++
	}
	controller = ctrl.Result{RequeueAfter: pollInterval}

	return controller, err
}

package controllers

import (
	"context"
	"fmt"
	"os"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	gitoperator "github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/git"
	"github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/resourcer"
	"github.com/eitco/cluster-api-addon-provider-cdk8s/controllers/synthesizer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	directory := fmt.Sprintf("/tmp/cdk8s-%s-%s-%s", cdk8sAppProxy.Namespace, cdk8sAppProxy.Name, branch)
	secretKey := cdk8sAppProxy.Spec.GitRepository.SecretKey
	secretName := cdk8sAppProxy.Spec.GitRepository.SecretRef
  var secretRef []byte
	var ok bool

	if cdk8sAppProxy.Spec.GitRepository.URL == "" {
		logger.Error(err, "GitRepository URL not specified")

		return ctrl.Result{}, err
	}

	if secretName != "" {
		logger.Info("SecretRef provided, fetching secret from cluster", "secretName", secretName)

		namespacedSecretKey := types.NamespacedName{
			Namespace: cdk8sAppProxy.Namespace,
			Name: secretName,
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

			return ctrl.Result{}, err
		}
		
		logger.Info("Parsing resources and synthing")
		parsedResources, err := synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
		if err != nil {
			logger.Error(err, "failed to synthesize resources")
			
			return ctrl.Result{}, err
		}

		logger.Info("Applying Synthed Code")
		err = resourcerImpl.Apply(ctx, cdk8sAppProxy, parsedResources, logger)
		if err != nil {
			logger.Error(err, "failed to apply resources")
		
			return ctrl.Result{}, err
		}
		logger.Info("Successfully applied resources")

		// ToDo: https://github.com/eitco/cluster-api-addon-provider-cdk8s/issues/13
		// Set Condition to ready
		// conditions.MarkTrue(cdk8sAppProxy, addonsv1alpha1.Cdk8sAppProxyReadyCondition)

		// Set the revision in the Cdk8sAppProxy status
		// cdk8sAppProxy.Status.Revision = 1
	}

	// parsedResources, err := synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
	// if err != nil {
	// 	logger.Error(err, "failed to synthesize resources")
	//
	// 	return ctrl.Result{}, err
	// }
	//
	// missingResources, err := resourcerImpl.Check(ctx, cdk8sAppProxy, parsedResources, logger)
	// if err != nil {
	// 	logger.Error(err, "failed to check resources")
	//
	// 	return ctrl.Result{}, err
	// }

	// if missingResources {
	// 	logger.Info("Missing resources detected, proceeding with reconciliation.")
	// 	parsedResources, err = synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
	// 	if err != nil {
	// 		logger.Error(err, "failed to synthesize resources")
	//
	// 		return ctrl.Result{}, err
	// 	}
	//
	// 	err = resourcerImpl.Apply(ctx, cdk8sAppProxy, parsedResources, logger)
	// 	if err != nil {
	// 		logger.Error(err, "failed to apply resources")
	//
	// 		return ctrl.Result{}, err
	// 	}
	//
		// ToDo: https://github.com/eitco/cluster-api-addon-provider-cdk8s/issues/13
		// Set Condition to ready
		// conditions.MarkTrue(cdk8sAppProxy, addonsv1alpha1.Cdk8sAppProxyReadyCondition)

		// Set the revision in the Cdk8sAppProxy status
		// cdk8sAppProxy.Status.Revision++

		return ctrl.Result{}, err
	}

	// hashChanges, err := gitImpl.Poll(repoURL, secretRef, branch, directory, logger)
	// if err != nil {
	// 	logger.Error(err, "Failed to poll git repository", "repoURL", repoURL, "branch", branch)
	//
	// 	return ctrl.Result{}, err
	// }
	//
	// if hashChanges {
	// 	logger.Info("Detected changes in git repository, proceeding with reconciliation.")
	//
	// 	if err := os.RemoveAll(directory); err != nil {
	// 		logger.Error(err, "Failed to clean up directory", "directory", directory)
	//
	// 		return ctrl.Result{}, err
	// 	}
	//
	// 	err = gitImpl.Clone(repoURL, secretRef, directory, logger)
	// 	if err != nil {
	// 		logger.Error(err, "Failed to clone git repository")
	//
	// 		return ctrl.Result{}, err
	// 	}
	//
	// 	parsedResources, err = synthImpl.Synthesize(directory, cdk8sAppProxy, logger, ctx)
	// 	if err != nil {
	// 		logger.Error(err, "failed to synthesize resources")
	//
	// 		return ctrl.Result{}, err
	// 	}
	//
	// 	err = resourcerImpl.Apply(ctx, cdk8sAppProxy, parsedResources, logger)
	// 	if err != nil {
	// 		logger.Error(err, "failed to apply resources")
	//
	// 		return ctrl.Result{}, err
	// 	}
	//
		// ToDo: https://github.com/eitco/cluster-api-addon-provider-cdk8s/issues/13
		// Set Condition to ready
		// conditions.MarkTrue(cdk8sAppProxy, addonsv1alpha1.Cdk8sAppProxyReadyCondition)

		// Set the revision in the Cdk8sAppProxy status
		// cdk8sAppProxy.Status.Revision++
	// }

	// return ctrl.Result{}, err
// }

package webhooks

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Cdk8sAppProxy struct{}

// log is for logging in this package.
var cdk8sappproxylog = logf.Log.WithName("cdk8sappproxy-resource")

func (c *Cdk8sAppProxy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-addons-cluster-x-k8s-io-v1alpha1-cdk8sappproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=cdk8sappproxies,verbs=create;update,versions=v1alpha1,name=default.#ToDo#cdk8sappproxy.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &Cdk8sAppProxy{}

//var _ webhook.Defaulter = &Cdk8sAppProxy{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (c *Cdk8sAppProxy) Default(ctx context.Context, obj runtime.Object) error {
	cdk8sappproxylog.Info("default", "name", c.Name)

	// Set the default git reference if not specified
	if c.Spec.GitRepository != nil && c.Spec.GitRepository.Reference == "" {
		c.Spec.GitRepository.Reference = "main"
	}

	if c.Spec.GitRepository != nil && c.Spec.GitRepository.ReferencePollInterval == nil {
		c.Spec.GitRepository.ReferencePollInterval = &metav1.Duration{
			Duration: 5 * time.Minute,
		}
	}

	// Set the default path if not specified
	if c.Spec.GitRepository != nil && c.Spec.GitRepository.Path == "" {
		c.Spec.GitRepository.Path = "."
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-addons-cluster-x-k8s-io-v1alpha1-cdk8sappproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=cdk8sappproxies,verbs=create;update,versions=v1alpha1,name=cdk8sappproxy.kb.io,admissionReviewVersions=v1

// var _ webhook.Validator = &Cdk8sAppProxy{}
var _ webhook.CustomValidator = &Cdk8sAppProxy{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (c *Cdk8sAppProxy) ValidateCreate(ctx context.Context, oldObj runtime.Object) (admission.Warnings, error) {
	cdk8sappproxylog.Info("validate create", "name", c.Name)

	return c.validateCdk8sAppProxy()
}

// ValidateUpdate implement webhook.CustomValidator so a webhook will be registered for the type.
func (c *Cdk8sAppProxy) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	cdk8sappproxylog.Info("validate update", "name", c.Name)

	return c.validateCdk8sAppProxy()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (c *Cdk8sAppProxy) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	cdk8sappproxylog.Info("validate delete", "name", c.Name)

	// No validation needed for delete
	return nil, nil
}

func (c *Cdk8sAppProxy) validateCdk8sAppProxy() (admission.Warnings, error) {
	var allErrs []error

	if c.Spec.GitRepository == nil {
		allErrs = append(allErrs, fmt.Errorf("gitRepository must be specified"))
	}

	// Validate GitRepository fields if specified
	if c.Spec.GitRepository != nil {
		if c.Spec.GitRepository.URL == "" {
			allErrs = append(allErrs, fmt.Errorf("gitRepository.url is required when gitRepository is specified"))
		}
	}

	if len(allErrs) > 0 {
		return nil, fmt.Errorf("validation failed: %v", allErrs)
	}

	return nil, nil
}

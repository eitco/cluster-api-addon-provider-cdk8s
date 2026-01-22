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

package v1alpha1

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var cdk8sappproxylog = logf.Log.WithName("cdk8sappproxy-resource")

func (c *Cdk8sAppProxy) SetupWebhookWithManager(mgr manager.Manager) error {
	w := new(cdk8sAppProxyWebhook)

	return controllerruntime.NewWebhookManagedBy(mgr, c). 
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-addons-cluster-x-k8s-io-v1alpha1-cdk8sappproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=cdk8sappproxies,verbs=create;update,versions=v1alpha1,name=cdk8sappproxy.kb.io,admissionReviewVersions=v1
type cdk8sAppProxyWebhook struct{}

var (
	_ admission.Validator[*Cdk8sAppProxy] = &cdk8sAppProxyWebhook{}
	_ admission.Defaulter[*Cdk8sAppProxy] = &cdk8sAppProxyWebhook{}
)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (*cdk8sAppProxyWebhook) Default(_ context.Context, obj *Cdk8sAppProxy) error {
	cdk8sappproxylog.Info("default", "name", obj.Name)

	// Defining the Reference is optional, so we set a default value.
	if obj.Spec.GitRepository.Reference == "" {
		obj.Spec.GitRepository.Reference = "main"
	}

	// Defining the Path is optional, so we set a default value.
	if obj.Spec.GitRepository.Path == "" {
		obj.Spec.GitRepository.Path = "."
	}

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-addons-cluster-x-k8s-io-v1alpha1-cdk8sappproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=addons.cluster.x-k8s.io,resources=cdk8sappproxies,verbs=create;update,versions=v1alpha1,name=cdk8sappproxy.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (*cdk8sAppProxyWebhook) ValidateCreate(_ context.Context, obj *Cdk8sAppProxy) (admission.Warnings, error) {
	var allErrs field.ErrorList

	cdk8sappproxylog.Info("validate create", "name", obj.Name) 

	if obj.Spec.GitRepository.URL == "" {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "GitRepository", "URL"),
				obj.Spec.GitRepository.URL, "GitRepository.URL must be specified"))
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("Cdk8sAppProxy").GroupKind(), obj.Name, allErrs)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (*cdk8sAppProxyWebhook) ValidateUpdate(_ context.Context, oldObjRaw, newObjRaw *Cdk8sAppProxy) (admission.Warnings, error) {
	var allErrs field.ErrorList

	cdk8sappproxylog.Info("validate update", "name", newObjRaw.Name)

	if !reflect.DeepEqual(newObjRaw.Spec.GitRepository.URL, oldObjRaw.Spec.GitRepository.URL) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "GitRepository", "URL"),
				newObjRaw.Spec.GitRepository.URL, "field is immutable"),
		)
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("Cdk8sAppProxy").GroupKind(), newObjRaw.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (*cdk8sAppProxyWebhook) ValidateDelete(_ context.Context, obj *Cdk8sAppProxy) (admission.Warnings, error) {
	cdk8sappproxylog.Info("validate delete", "name", obj.Name)

	// ToDo: Define delete validation, if we need any. Needs to be decided.

	return nil, nil
}

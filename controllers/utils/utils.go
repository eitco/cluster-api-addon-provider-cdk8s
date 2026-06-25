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

package utils

import (
	"context"
	"fmt"

	addonsv1alpha1 "github.com/eitco/cluster-api-addon-provider-cdk8s/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func FetchSecret(ctx context.Context, c client.Client, namespace string, spec *addonsv1alpha1.GitRepositorySpec, logs logr.Logger) (secretRef []byte, err error) {
	if spec == nil || spec.SecretRef == "" {
		err = fmt.Errorf("secret reference is empty")
		logs.Error(err, "secret reference is empty")

		return secretRef, err
	}

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      spec.SecretRef,
	}

	if err = c.Get(ctx, secretKey, secret); err != nil {
		logs.Error(err, "failed to get secret", "secret", spec.SecretRef)

		return secretRef, err
	}

	secretRef, ok := secret.Data[spec.SecretKey]
	if !ok {
		err = fmt.Errorf("secret %q does not contain key %q", spec.SecretRef, spec.SecretKey)
		logs.Error(err, "secret does not contain key", "secret", spec.SecretRef, "key", spec.SecretKey)

		return secretRef, err
	}
	logs.Info("found secret", "secret", spec.SecretRef, "with key", spec.SecretKey)

	return secretRef, err
}

// FetchKnownHosts returns the SSH known_hosts entry stored under spec.KnownHostsKey within
// the secret referenced by spec.SecretRef. It returns nil (and no error) when no known_hosts
// key is configured, leaving host-key verification to the controller's baked-in known_hosts.
func FetchKnownHosts(ctx context.Context, c client.Client, namespace string, spec *addonsv1alpha1.GitRepositorySpec, logs logr.Logger) (knownHosts []byte, err error) {
	if spec == nil || spec.SecretRef == "" || spec.KnownHostsKey == "" {
		return knownHosts, err
	}

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      spec.SecretRef,
	}

	if err = c.Get(ctx, secretKey, secret); err != nil {
		logs.Error(err, "failed to get secret", "secret", spec.SecretRef)

		return knownHosts, err
	}

	knownHosts, ok := secret.Data[spec.KnownHostsKey]
	if !ok {
		err = fmt.Errorf("secret %q does not contain known_hosts key %q", spec.SecretRef, spec.KnownHostsKey)
		logs.Error(err, "secret does not contain known_hosts key", "secret", spec.SecretRef, "key", spec.KnownHostsKey)

		return knownHosts, err
	}
	logs.Info("found known_hosts", "secret", spec.SecretRef, "with key", spec.KnownHostsKey)

	return knownHosts, err
}

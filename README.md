<a href="https://cluster-api.sigs.k8s.io"><img alt="capi" src="./logos/kubernetes-cluster-logos_final-02.svg" width="160x" /></a>
<p>
<a href="https://godoc.org/sigs.k8s.io/cluster-api"><img src="https://godoc.org/sigs.k8s.io/cluster-api?status.svg"></a>

# Cluster API Add-on Provider for Cdk8s

## Cdk8sAppProxy CRD

The `Cdk8sAppProxy` CustomResourceDefinition (CRD) is used to manage the deployment of cdk8s applications to workload clusters. It allows users to specify the source of the cdk8s application, any input values, and the target clusters for deployment.

### Example Manifest

An example of a `Cdk8sAppProxy` manifest can be found in [`examples/cdk8sappproxy_sample-go.yaml`](./examples/cdk8sappproxy_sample-go.yaml). Below is a snippet:

```yaml
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: Cdk8sAppProxy
metadata:
  name: cdk8s-sample-app-go
  namespace: default
spec:
  gitRepository:
    url: "https://github.com/eitco/cluster-api-addon-provider-cdk8s"
    reference: "main"
    path: "examples/cdk8s-sample-deployment"
  clusterSelector: {} 
    # matchLabels:
      # environment: development
```

If you want to use a public or private repository for your deployments, you can find some guidance [here](./docs/private-repositories.md)

### Cdk8sAppProxySpec Fields
```
// GitRepositorySpec defines the desired state of a Git repository source.
type GitRepositorySpec struct {
	// URL is the git repository URL.
	// If the Repository is private,
	// Valid options are: 'HTTP', 'HTTPS', and 'git@...' 
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Reference (optional) defines the branch, tag or hash which CAAPC
	// will pull from. If left empty, defaults to 'main'.
	// +kubebuilder:validation:optional
	Reference string `json:"reference,omitempty"`

	// Path (optional) is the path within the repository where the cdk8s application is located.
	// Defaults to the root of the repository.
	// +kubebuilder:validation:optional
	Path string `json:"path,omitempty"`

	// SecretRef references to a secret with the
	// needed token, used to pull from a private repository.
	// Valid options are SSHKeys and PAT Tokens.
	// +kubebuilder:validation:optional
	SecretRef string `json:"secretRef,omitempty"`

	// SecretKey is the key within the SecretRef secret.
	// +kubebuilder:validation:optional
	SecretKey string `json:"secretKey,omitempty"`
}
```

## Examples

Examples of `Cdk8sAppProxy` usage can be found in the `/examples` directory in this repository.

-   [`examples/cdk8sappproxy_sample-go.yaml`](./examples/cdk8sappproxy_sample-go.yaml): This example demonstrates how to deploy a sample cdk8s application from a public Git repository to clusters matching a specific label selector. It shows the usage of the `gitRepository` field.
-   [`examples/cdk8sappproxy_sample-typescript.yaml`](./examples/cdk8sappproxy_sample-typescript.yaml): This directory contains a sample cdk8s application written in Typescript, which also generates a kustomization file.

### Supported Platforms:

`amd64 arm64 ppc64le`

`s390` and `arm` is currently not supported.

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

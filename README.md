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
    referencePollInterval: "5m"
    path: "examples/cdk8s-sample-deployment"
  clusterSelector: {} # Matches all clusters in the same namespace
    # matchLabels:
      # environment: development
```

### Cdk8sAppProxySpec Fields

- **gitRepository**: (Optional) Specifies the Git repository for the cdk8s application. `gitRepository` must be specified.
    - **url**: (Required) The Git repository URL.
    - **reference**: (Required) The Git reference (branch, tag, or commit) to check out.
    - **referencePollInterval**: (Optional) The interval at which the controller checks for changes in the Git repository. Defaults to `5m`.
    - **path**: (Required) The path within the repository where the cdk8s application is located. Defaults to the root.
- **clusterSelector**: (Required) A `metav1.LabelSelector` that specifies which workload clusters the cdk8s application should be deployed to. The controller will watch for clusters matching this selector in the same namespace as the `Cdk8sAppProxy` resource.

## Examples

Examples of `Cdk8sAppProxy` usage can be found in the `/examples` directory in this repository.

-   [`examples/cdk8sappproxy_sample-go.yaml`](./examples/cdk8sappproxy_sample-go.yaml): This example demonstrates how to deploy a sample cdk8s application from a public Git repository to clusters matching a specific label selector. It shows the usage of the `gitRepository` field.
-   [`examples/cdk8sappproxy_sample-typescript.yaml`](./examples/cdk8sappproxy_sample-typescript.yaml): This directory contains a sample cdk8s application written in Typescript, which also generates a kustomization file.

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

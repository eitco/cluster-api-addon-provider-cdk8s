<a href="https://cluster-api.sigs.k8s.io"><img alt="capi" src="./logos/kubernetes-cluster-logos_final-02.svg" width="160x" /></a>
<p>
<a href="https://godoc.org/sigs.k8s.io/cluster-api"><img src="https://godoc.org/sigs.k8s.io/cluster-api?status.svg"></a>
<!-- join kubernetes slack channel for cluster-api -->
<a href="http://slack.k8s.io/">
<img src="https://img.shields.io/badge/join%20slack-%23cluster--api-brightgreen"></a>
</p>

# Cluster API Add-on Provider for Cdk8s

## ✨ What is Cluster API Add-on Provider for Cdk8s?

Cluster API Add-on Provider for Cdk8s extends Cluster API by managing the installation, configuration, upgrade, and deletion of cluster add-ons using cdk8s applications. This provider is based on the [Cluster API Add-on Orchestration Proposal](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20220712-cluster-api-addon-orchestration.md), a larger effort to bring orchestration for add-ons to CAPI by using existing package management tools.

This project is a concrete implementation of a ClusterAddonProvider, a pluggable component to be deployed on the Management Cluster. An add-on provider component acts as a broker between Cluster API and a package management tool (in this case, cdk8s).

The aims of the ClusterAddonProvider project are as follows:

#### Goals

- Design a solution for orchestrating Cluster add-ons.
- Leverage the capabilities of cdk8s for defining and synthesizing Kubernetes manifests.
- Make add-on management in Cluster API modular and pluggable.
- Make it clear for developers how to build a Cluster API Add-on Provider based on cdk8s or other tools.

#### Non-goals

- Implement a new, full-fledged package management tool in Cluster API.
- Provide a mechanism for altering, customizing, or dealing with single Kubernetes resources defining a Cluster add-on, i.e. Deployments, Services, ServiceAccounts. Cluster API should treat add-ons as opaque components and delegate all the operations impacting add-on internals to cdk8s.
- Expect users to use a specific package management tool (though this provider focuses on cdk8s).
- Implement a solution for installing add-ons on the management cluster itself.

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
  clusterSelector: {}
          # matchLabels:
  # environment: development
```

### Cdk8sAppProxySpec Fields

- **gitRepository**: (Optional) Specifies the Git repository for the cdk8s application. `gitRepository` must be specified.
    - **url**: (Required) The Git repository URL.
    - **reference**: (Optional) The Git reference (branch, tag, or commit) to check out.
    - **path**: (Optional) The path within the repository where the cdk8s application is located. Defaults to the root.
- **values**: (Optional) A string containing values to be passed to the cdk8s application, typically in YAML or JSON format. This can be used to customize the deployment. The controller supports Go templating within this field, allowing dynamic values based on cluster properties (e.g., `{{ .Cluster.spec.clusterNetwork.apiServerPort }}`).
- **clusterSelector**: (Required) A `metav1.LabelSelector` that specifies which workload clusters the cdk8s application should be deployed to. The controller will watch for clusters matching this selector in the same namespace as the `Cdk8sAppProxy` resource.

## Examples

Examples of `Cdk8sAppProxy` usage can be found in the `/examples` directory in this repository.

-   [`examples/cdk8sappproxy_sample-go.yaml`](./examples/cdk8sappproxy_sample-go.yaml): This example demonstrates how to deploy a sample cdk8s application from a public Git repository to clusters matching a specific label selector. It shows the usage of the `gitRepository` field.
-   [`examples/cdk8sappproxy_sample-typescript.yaml`](./examples/cdk8sappproxy_sample-typescript.yaml): This directory contains a sample cdk8s application written in Typescript, which also generates a kustomization file.

## 🤗 Community, discussion, contribution, and support

Cluster API Add-on Provider for Cdk8s is developed as a part of the [Cluster API project](https://github.com/kubernetes-sigs/cluster-api). As such, it will share the same communication channels and meeting as Cluster API.

This work is made possible due to the efforts of users, contributors, and maintainers. If you have questions or want to get the latest project news, you can connect with us in the following ways:

- Chat with us on the Kubernetes [Slack](http://slack.k8s.io/) in the [#cluster-api](https://kubernetes.slack.com/archives/C8TSNPY4T) channel
- Subscribe to the [SIG Cluster Lifecycle](https://groups.google.com/forum/#!forum/kubernetes-sig-cluster-lifecycle) Google Group for access to documents and calendars

Pull Requests and feedback on issues are very welcome!
See the [issue tracker](https://github.com/eitco/cluster-api-addon-provider-cdk8s/issues) if you're unsure where to start, especially the [Good first issue](https://github.com/eitco/cluster-api-addon-provider-cdk8s/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22) and [Help wanted](https://github.com/eitco/cluster-api-addon-provider-cdk8s/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) tags, and
also feel free to reach out to discuss.

See also our [contributor guide](CONTRIBUTING.md) and the Kubernetes [community page](https://kubernetes.io/community/) for more details on how to get involved.

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

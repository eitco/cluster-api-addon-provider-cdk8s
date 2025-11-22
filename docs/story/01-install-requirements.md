# List of requirements

In order to fully use a preview environment, you will need the following on your kubernetes cluster:

1. Cluster-API Core Componentes
2. CAAPC Provider
3. [CAPV (vcluster) Provider](https://github.com/loft-sh/cluster-api-provider-vcluster)
4. (optional) Using a secret store (Vault, openBao) with a the [external secrets operator](https://external-secrets.io/latest/)
5. Git Repository for your cdk8s deployment
6. ArgoCD Controller with the PullRequest Generator (which we currently still rely on. #91)

This guide assumes, that you want to use openBao as your secret-store and the external-secrets-operator.

## Installing the requirements

### CAPI, CAPV and CAAPC

`clusterctl init --infrastructure vcluster:v0.3.0-alpha.2 --addon eitco-cdk8`

### ArgoCD ApplicationSet PullRequest Generator

We use the following AppSet to watch for PullRequest on the target Repository.
If there is any, the PullRequest Generator will start creating a vcluster, Cdk8sAppProxy resource on our cluster, and the the secret creation, using ESO.

```
---
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: appset-pr-generator
  namespace: argocd
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - pullRequest:
      github:
        owner: PatrickLaabs
        repo: cdk8s-sample-deployment
        tokenRef:
          secretName: gh-token
          key: token
    requeueAfterSeconds: 30
  template:
    metadata:
      name: 'preview-{{.branch}}-{{.number}}'
    spec:
      source:
        repoURL: '<REPO-URL>'
        targetRevision: 'main'
        path: deployments/preview-environments/vcluster-generator
        kustomize:
          nameSuffix: "previewcluster-{{.number}}"          
          patches:
          - patch: |-
              apiVersion: cluster.x-k8s.io/v1beta2
              kind: Cluster
              metadata:
                name: vcluster
                labels:
                  preview: "vclusterpreviewcluster-{{.number}}"
              spec:
                controlPlaneRef:
                  name: "vclusterpreviewcluster-{{.number}}"
                infrastructureRef:
                  name: "vclusterpreviewcluster-{{.number}}"
            target:
              kind: Cluster
              name: vcluster
          - patch: |-
              apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
              kind: VCluster
              metadata:
                name: vcluster
            target:
              kind: VCluster
              name: vcluster
          - patch: |-
              apiVersion: addons.cluster.x-k8s.io/v1alpha1
              kind: Cdk8sAppProxy
              metadata:
                name: cdk8s-private-test
              spec:
                gitRepository:
                  reference: "{{.branch}}"
                clusterSelector:
                  matchLabels:
                    preview: "vclusterpreviewcluster-{{.number}}"
            target:
              kind: Cdk8sAppProxy
              name: cdk8s-private-test
      project: default
      destination:
        name: in-cluster
        namespace: "vclusterpreviewcluster-{{.number}}"
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        managedNamespaceMetadata:
          labels:
            argocd.argoproj.io/instance: "preview-{{.branch}}-{{.number}}"
        syncOptions:
          - CreateNamespace=true
      ignoreDifferences:
        - jsonPointers:
            - /controlPlaneEndpoint
          kind: VCluster
```

### Secret creation using external-secrets-operator

Here is an example of an ExternalSecret, which will create a Kubernetes Secret.
```
---
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: gh-token
spec:
  refreshInterval: 3m
  secretStoreRef:
    kind: ClusterSecretStore
    name: oracle-vault
  target:
    name: gh-token
    creationPolicy: Owner
    deletionPolicy: Delete
    template:
      engineVersion: v2
      data:
        token: "{{ .token }}"
  data:
    - secretKey: token
      remoteRef:
        key: gh-token
        version: CURRENT
```

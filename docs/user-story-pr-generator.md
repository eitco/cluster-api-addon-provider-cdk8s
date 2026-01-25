# User Story: Automated Preview Environments with PR Generator

## Background
As a developer working on a large-scale Kubernetes application, I want to ensure that every change I make is tested in a real environment before it gets merged into the main branch. Manually creating and cleaning up these environments is time-consuming and error-prone.

## The Goal
I want an automated system that:
1. Detects when I open a Pull Request in my GitHub repository.
2. Automatically deploys a "Preview Environment" containing the changes from my PR branch.
3. Updates the environment whenever I push new commits to the PR.
4. Cleans up the environment automatically when the PR is merged or closed.

## Implementation with CAAPC PR Generator

With the `Cdk8sAppProxyGenerator`, I can achieve this easily.

### 1. Configure the Generator
I create a `Cdk8sAppProxyGenerator` resource in my management cluster. This resource tells CAAPC which repository to watch and how to create the preview environments.

```yaml
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: Cdk8sAppProxyGenerator
metadata:
  name: web-app-pr-generator
  namespace: default
spec:
  pollInterval: 2m  # Check for new PRs every 2 minutes
  source:
    url: https://github.com/my-org/web-app
    secretRef: github-pat
    secretKey: token
  template:
    spec:
      clusterSelector: {}
        # matchLabels:
        #   environment: preview-cluster
```

### 2. Automatic Detection
The CAAPC controller periodically polls the GitHub API. When I open PR #42 named `feature-express-update`, the controller detects it.

### 3. Resource Generation
The controller automatically generates a new `Cdk8sAppProxy` resource:

```yaml
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: Cdk8sAppProxy
metadata:
  name: web-app-pr-generator-pr-42
  namespace: default
  labels:
    addons.cluster.x-k8s.io/generator-name: web-app-pr-generator
    addons.cluster.x-k8s.io/pr-number: "42"
  ownerReferences:
    - apiVersion: addons.cluster.x-k8s.io/v1alpha1
      kind: Cdk8sAppProxyGenerator
      name: web-app-pr-generator
spec:
  gitRepository:
    url: https://github.com/my-org/web-app
    reference: feature-express-update  # Automatically set to the PR branch
    secretRef: github-pat
    secretKey: token
  clusterSelector:
    matchLabels:
      environment: preview-cluster
```

### 4. Deployment
The existing CAAPC controller then picks up this new `Cdk8sAppProxy`, clones the `feature-express-update` branch, synthesizes the cdk8s code, and applies it to the target cluster.

### 5. Cleanup
Once my PR is merged and closed, the next poll cycle will notice the PR is no longer open. The generator will then ensure the associated `Cdk8sAppProxy` is deleted, which in turn (via finalizers) cleans up the resources in the preview cluster.

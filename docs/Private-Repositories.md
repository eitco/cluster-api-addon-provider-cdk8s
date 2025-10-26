# Using CAAPC with a private Repository

Disclaimer:
This guide assumes, that you have a running management CAPI cluster.
Depending on your labeled cluster, you might want to use the vcluster provider, to install a workload cluster ontop of your management cluster.

---
The private Repository - which we are going to use - is based on [this](https://github.com/PatrickLaabs/cdk8s-sample-deployment-public) public Repository. 
You might want to set up you own one.

### Set up a github-token

Create a kubernetes secret, which holds your ssh private key, to connect to your private repository.We suggest using a secret store, like openBao - or such - with the external-secret-operator, to makelive easier.

This secret should be formated like this:

```
---
apiVersion: v1
data:
  api-token: <YOUR PRIVATE KEY> 
kind: Secret
metadata:
  name: github-token
  namespace: caapc-system
type: Opaque

```

### Cdk8sAppProxy Resource example
```
---
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: Cdk8sAppProxy
metadata:
  name: cdk8s-private-test
  namespace: default
spec:
  gitRepository:
    url: "git@github.com:PatrickLaabs/cdk8s-sample-deployment.git" 
    reference: "main"
    path: "."
    secretRef: github-token
    secretKey: api-token
  clusterSelector: {}

```

To learn more about a specific key, please refer to our API implementation, or use `kubectl explain cdk8sappproxy`.

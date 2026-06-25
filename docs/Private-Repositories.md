# Using CAAPC with a private Repository

Disclaimer:
Depending of your kind of cluster (Cluster-API managed, or not), you might want to use the vcluster provider, to run a cluster-api compatible kubernetes cluster on top of your kubernetes cluster.

---
The private Repository - which we are going to use - is based on [this](https://github.com/PatrickLaabs/cdk8s-sample-deployment-public) public Repository. 
You might want to set up you own.

### Set up a github-token
Create a kubernetes secret, which holds your ssh private key, to connect to your private repository.
We suggest using a secret store, like openBao - or such - with the external-secret-operator.

For self-hosted SSH servers whose host key is not baked into the controller image, also store a
`known_hosts` entry in the same secret. Generate it with `ssh-keyscan`, e.g. for a Bitbucket
Server on a custom port:
```
ssh-keyscan -p 7999 git.example.de
```

The secret - that holds your token (and optionally the known_hosts entry) - should be formated in a form like this:
```
---
apiVersion: v1
data:
  api-token: <YOUR PRIVATE KEY>
  # optional, only needed for self-hosted hosts - base64 of the ssh-keyscan output:
  known-hosts: <YOUR KNOWN_HOSTS ENTRY>
kind: Secret
metadata:
  name: github-token
  namespace: default
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
    knownHostsKey: known-hosts
  clusterSelector: {}
```

To learn more about a specific key, please refer to our API implementation, or use `kubectl explain cdk8sappproxy`.

### Authentication support

- **SSH** (`git@host:...` or `ssh://git@host:port/...`): works against any host, including self-hosted
  servers. For self-hosted hosts whose key is not baked into the controller image, provide the host key
  via `knownHostsKey` as shown above. Store an unencrypted PEM private key in `secretKey`.
- **HTTPS** (`https://host/...`): token auth is sent as HTTP Basic Auth with the username `oauth2`,
  which covers GitHub and GitLab personal access tokens. Self-hosted **Bitbucket Data Center** is
  **not supported over HTTPS** (it expects a different username / Bearer scheme) — use SSH for Bitbucket.

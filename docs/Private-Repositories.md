# Using CAAPC with a private Repository

Disclaimer:
Depending of your kind of cluster (Cluster-API managed, or not), you might want to use the vcluster provider, to run a cluster-api compatible kubernetes cluster on top of your kubernetes cluster.

---
The private Repository - which we are going to use - is based on [this](https://github.com/PatrickLaabs/cdk8s-sample-deployment-public) public Repository.
You might want to set up you own.

## How authentication is selected

CAAPC does **not** have a field to pick the authentication method. The method is derived
**from the `url` scheme** of your `gitRepository`. This is the single most important thing
to understand, because it dictates what kind of credential you must put in your secret:

| `url` form                                  | Auth method used   | What `secretKey` must contain                |
|---------------------------------------------|--------------------|----------------------------------------------|
| `https://host/owner/repo.git`               | HTTP Basic (token) | a **Personal Access Token (PAT)**            |
| `git@host:owner/repo.git`                   | SSH                | an **unencrypted PEM private key**           |
| `ssh://git@host:port/owner/repo.git`        | SSH                | an **unencrypted PEM private key**           |

The credential is always read from the **same** `secretRef` / `secretKey` pair — its
*meaning* changes with the URL scheme. An `https://` URL with a private key in the secret
(or a `git@` URL with a PAT) will fail authentication.

Pick **one** of the two options below and make sure the URL scheme and the secret content match.

---

## Option A — HTTPS with a Personal Access Token

Use this for `https://...` URLs. The token is sent as HTTP Basic Auth with the username
`oauth2`, which works for **GitHub** and **GitLab** personal access tokens.

> Self-hosted **Bitbucket Data Center** is **not supported over HTTPS** (it expects a
> different username / Bearer scheme). Use Option B (SSH) for Bitbucket.

The token needs **read** access to the repository:
- GitHub classic PAT: the `repo` scope. Fine-grained PAT: **Contents: Read** (+ **Metadata: Read**),
  the repo selected, and — for org repos — the org must allow the token (and SSO authorized).
- GitLab: a PAT with `read_repository` scope.

### Secret

`stringData` lets Kubernetes base64-encode for you, so you can paste the token verbatim:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: git-credentials
  namespace: default
type: Opaque
stringData:
  token: <YOUR PERSONAL ACCESS TOKEN>
```

### Cdk8sAppProxy

```yaml
---
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: Cdk8sAppProxy
metadata:
  name: cdk8s-private-test
  namespace: default
spec:
  gitRepository:
    url: "https://github.com/PatrickLaabs/cdk8s-sample-deployment.git"
    reference: "main"
    path: "."
    secretRef: git-credentials
    secretKey: token
  clusterSelector: {}
```

---

## Option B — SSH with a private key

Use this for `git@host:...` or `ssh://git@host:port/...` URLs. This is the recommended
option for **self-hosted** servers (including Bitbucket Data Center).

The `secretKey` must hold an **unencrypted** PEM private key (passphrase-protected keys are
not supported). The matching **public** key must be registered as a deploy key / SSH key
with read access on the server.

For self-hosted hosts whose host key is not baked into the controller image, you must also
provide the server's host key via `knownHostsKey`, otherwise host-key verification fails.
Generate the entry with `ssh-keyscan` (note the custom port for e.g. Bitbucket Server):

```
ssh-keyscan -p 7999 git.example.com
```

### Secret

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: git-credentials
  namespace: default
type: Opaque
stringData:
  ssh-privatekey: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    <YOUR PRIVATE KEY>
    -----END OPENSSH PRIVATE KEY-----
  # optional, only for self-hosted hosts - the ssh-keyscan output:
  known-hosts: "[git.example.com]:7999 ssh-rsa AAAAB3Nz..."
```

Or create it directly from files, which avoids any encoding mistakes:

```
kubectl create secret generic git-credentials -n default \
  --from-file=ssh-privatekey=/path/to/id_ed25519 \
  --from-file=known-hosts=<(ssh-keyscan -p 7999 git.example.com)
```

### Cdk8sAppProxy

```yaml
---
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: Cdk8sAppProxy
metadata:
  name: cdk8s-private-test
  namespace: default
spec:
  gitRepository:
    # public host (github.com / gitlab.com): knownHostsKey not needed
    url: "git@github.com:PatrickLaabs/cdk8s-sample-deployment.git"
    # self-hosted host on a custom port: use ssh:// with the port and set knownHostsKey
    # url: "ssh://git@git.example.com:7999/uvas/caapc-deployments.git"
    reference: "main"
    path: "."
    secretRef: git-credentials
    secretKey: ssh-privatekey
    knownHostsKey: known-hosts   # only needed for self-hosted hosts
  clusterSelector: {}
```

> When using `ssh://...` with a custom port, make sure the port in the URL matches the port
> in your `known-hosts` entry (e.g. `[git.example.com]:7999`), or verification will fail.

---

## Field reference

| Field           | Required | Description                                                                 |
|-----------------|----------|-----------------------------------------------------------------------------|
| `url`           | yes      | Repository URL. Its scheme selects the auth method (see table above).       |
| `reference`     | yes      | Branch (or ref) to track.                                                    |
| `path`          | yes      | Path within the repository to synthesize.                                    |
| `secretRef`     | for private repos | Name of the secret holding the credential, in the proxy's namespace.|
| `secretKey`     | for private repos | Key within `secretRef` holding the PAT (HTTPS) or private key (SSH). |
| `knownHostsKey` | self-hosted SSH | Key within `secretRef` holding the `ssh-keyscan` known_hosts entry.   |

To learn more about a specific key, please refer to our API implementation, or use `kubectl explain cdk8sappproxy`.

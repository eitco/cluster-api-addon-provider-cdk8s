# CLAUDE.md

Guidance for working in this repository.

## What this is

A Cluster API Addon Provider that synthesizes [cdk8s](https://cdk8s.io/) apps from a Git
repository and applies the resulting Kubernetes manifests to target (workload) clusters. It is
a controller-runtime/Kubebuilder project.

- Module: `github.com/eitco/cluster-api-addon-provider-cdk8s`, Go `1.26.4`.
- API group: `addons.cluster.x-k8s.io/v1alpha1`.
- Kinds: **`Cdk8sAppProxy`** (clone a repo, synthesize, deploy to selected clusters) and
  **`Cdk8sAppProxyGenerator`** (open per-pull-request proxies).

## Build / test / lint

```bash
make manager          # build the controller binary into ./bin
make test             # unit + envtest integration tests (sets up envtest assets)
make lint             # golangci-lint (config: .golangci.yaml) + Dockerfile lint
make generate         # regenerate CRDs/RBAC/deepcopy after API changes (controller-gen)
make verify           # boilerplate + shellcheck + modules + gen checks (CI gate)
```

### Running tests directly (important)

`go test ./...` **fails** unless envtest binaries are present — the integration suite
(`controllers/suite_test.go`) starts a real `kube-apiserver`/`etcd` via `KUBEBUILDER_ASSETS`.
The error looks like `fork/exec /usr/local/kubebuilder/bin/kube-apiserver: no such file`.

- Full suite: use **`make test`** (it runs `setup-envtest` and exports `KUBEBUILDER_ASSETS`).
- Pure unit packages (no apiserver) run fine with plain `go test`, e.g.:
  ```bash
  go test ./controllers/git/... ./controllers/resourcer/... ./controllers/synthesizer/...
  ```
  Prefer this tight loop when iterating on those packages.

After editing anything under `api/`, run `make generate` or `make verify-gen` will fail in CI.
Every Go/shell file needs the Apache license header (`make verify-boilerplate`).

## Architecture

Reconcile pipeline (`controllers/controller.go`, `Reconciler.Reconcile`):
1. Fetch credentials from the referenced secret (`controllers/utils`).
2. Clone / poll the Git repo (`controllers/git`).
3. Synthesize cdk8s output into manifests (`controllers/synthesizer`).
4. Apply manifests to target clusters chosen by the cluster selector (`controllers/resourcer`).

`controllers/generator_controller.go` (`GeneratorReconciler`) lists open PRs via
`controllers/git/provider_client.go` (GitHub/GitLab/Bitbucket REST) and manages a proxy per PR.

### `controllers/git` — the package we change most

- `git.go`: `Implementer` (Clone/Poll/Hash/CheckAccess) + `getAuth`. Auth transport comes from
  go-git. `provider_client.go`: REST clients for PR listing (separate from clone auth).
- **Auth method is derived from the URL scheme**, not a config field (`getURLType`):
  - `https://…` → `http.BasicAuth` with username `oauth2` → secret must hold a **PAT**
    (works for GitHub/GitLab; self-hosted **Bitbucket Data Center HTTPS is not supported**).
  - `git@…` / `ssh://…` → `ssh.NewPublicKeys` → secret must hold an **unencrypted PEM private key**.
  - The *same* `secretRef`/`secretKey` field holds either, depending on the scheme. `getAuth`
    fail-fasts on a mismatch via `looksLikePrivateKey` (e.g. a key on an HTTPS URL).
- SSH host-key verification is fail-closed. For self-hosted hosts, the server key must be
  supplied via `knownHostsKey` (a key in the same secret holding `ssh-keyscan` output);
  otherwise it falls back to the image's baked-in known_hosts. See
  `docs/Private-Repositories.md` for the full user-facing setup.

## Conventions

- **Named returns** are used throughout (`func ... (auth transport.AuthMethod, err error)`).
  Match the surrounding style.
- **Don't log-and-swallow.** When a branch fails, return a real `error` (e.g.
  `fmt.Errorf(...)`); a bare `logs.Error(nil, msg)` that returns a nil error hides the failure
  from the caller. Logging *and* returning is the existing pattern.
- **Tests**: table-driven, internal (`package git`), and **offline** — no network or real
  credentials. Generate keys in-test (`newTestSigner`, `newTestPrivateKeyPEM`) and exercise
  callbacks/`getAuth` directly. Keep new unit tests in this style so they run without envtest.
- Follow TDD for behavior changes here: one failing test → minimal code → repeat.

# Releasing

Our release workflow is - currently - hand-crafted.

First, edit the Makefile to bump the version:

```
TAG ?= v1.0.0-alpha.10
```

and build the containers:

`make docker-build-all docker-push-all`

Next, run:

`make release-manifests`

`make release-metadata`

Lastly:

Create a Release on Github, and upload the YAMLs to that release.

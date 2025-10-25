# syntax=docker/dockerfile:1.4

# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the manager binary
# Run this with docker build --build-arg builder_image=<golang:x.y.z>
ARG builder_image
ARG deployment_base_image
ARG deployment_base_image_tag
ARG goprivate

# Build step only to fetch the ssh_known_hosts
FROM ${deployment_base_image}:${deployment_base_image_tag} AS sshbuilder
WORKDIR /ssh

RUN apk add --no-cache openssh=10.0_p1-r9 openssh-client=10.0_p1-r9

COPY ./hack/update-ssh-known-hosts.sh ./

# Known Hosts
RUN ./update-ssh-known-hosts.sh

# Build architecture
ARG ARCH

# Ignore Hadolint rule "Always tag the version of an image explicitly."
# It's an invalid finding since the image is explicitly set in the Makefile.
# https://github.com/hadolint/hadolint/wiki/DL3006
# hadolint ignore=DL3006
FROM ${builder_image} AS builder
WORKDIR /workspace

# Run this with docker build --build-arg goproxy=$(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
# Run this with docker build --build-arg package=./controlplane/kubeadm or --build-arg package=./bootstrap/kubeadm
ENV GOPROXY=$goproxy
ENV GOPRIVATE=$goprivate

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=secret,id=netrc,required=false,target=/root/.netrc \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the sources
COPY ./ ./

# Cache the go build into the Go’s compiler cache folder so we take benefits of compiler caching across docker build calls
RUN --mount=type=secret,id=netrc,required=false,target=/root/.netrc \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build .

# Build
ARG package=.
ARG ARCH
ARG ldflags

# Do not force rebuild of up-to-date packages (do not use -a) and use the compiler cache folder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} \
    go build -trimpath -ldflags "${ldflags} -extldflags '-static'" \
    -o manager ${package}

# Production image
FROM ${deployment_base_image}:${deployment_base_image_tag}

# Build architecture - redeclare for this stage
ARG ARCH

# Set shell with pipefail option for better error handling
SHELL ["/bin/sh", "-o", "pipefail", "-c"]

RUN apk add --no-cache ca-certificates=20250911-r0 curl=8.14.1-r2 nodejs=22.16.0-r2 npm=11.3.0-r1 \
    && npm install -g cdk8s-cli@2.202.3 \
    && curl -fsSL -o go1.25.0.linux-${ARCH}.tar.gz https://go.dev/dl/go1.25.0.linux-${ARCH}.tar.gz \
    && tar -C /usr/local -xzf go1.25.0.linux-${ARCH}.tar.gz \
    && rm go1.25.0.linux-${ARCH}.tar.gz \
    && rm -rf /tmp/*

# Set Go environment variables
ENV PATH=$PATH:/usr/local/go/bin
ENV GOROOT=/usr/local/go

WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=sshbuilder /ssh/ssh_known_hosts /etc/ssh/ssh_known_hosts

# Create non-root user
RUN adduser -u 65532 -D -h /home/nonroot -s /bin/sh nonroot
   
# Switch back to non-root user (this line should already exist)
# USER root # This was part of the removed direct install, ensure it's not re-added here unless needed for COPY permissions
USER 65532
ENTRYPOINT ["/manager"]

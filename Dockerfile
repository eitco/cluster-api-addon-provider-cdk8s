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
FROM --platform=$TARGETPLATFORM ${deployment_base_image}:${deployment_base_image_tag} AS sshbuilder
ARG TARGETPLATFORM

WORKDIR /ssh

RUN apk add --no-cache openssh=10.2_p1-r0 openssh-client=10.2_p1-r0

COPY ./hack/update-ssh-known-hosts.sh ./

# Known Hosts
RUN ./update-ssh-known-hosts.sh

FROM --platform=$BUILDPLATFORM ${builder_image} AS builder
ARG TARGETPLATFORM
ARG TARGETARCH

WORKDIR /workspace

# Run this with docker build --build-arg goproxy=$(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
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
ARG ldflags

# Do not force rebuild of up-to-date packages (do not use -a) and use the compiler cache folder
# hadolint ignore=SC2086
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "${ldflags} -extldflags '-static'" \
    -o manager ${package}

# Go Runtime Builder
FROM --platform=$TARGETPLATFORM ${deployment_base_image}:${deployment_base_image_tag} AS go_runtime_builder
ARG TARGETARCH
ARG go_version
ARG curl_version
ARG xz_version
ARG tar_version

RUN apk add --no-cache curl=8.17.0-r1 tar=1.35-r4 xz=5.8.2-r0
RUN curl -fsSL -o go1.25.6.linux-${TARGETARCH}.tar.gz https://go.dev/dl/go1.25.6.linux-${TARGETARCH}.tar.gz \
    && tar -C /usr/local -xzf go1.25.6.linux-${TARGETARCH}.tar.gz \
    && rm go1.25.6.linux-${TARGETARCH}.tar.gz 

# Production image
FROM --platform=$TARGETPLATFORM ${deployment_base_image}:${deployment_base_image_tag}
ARG TARGETPLATFORM

WORKDIR /

RUN apk add --no-cache nodejs=24.13.0-r1 npm=11.6.3-r0 \
    && npm install -g cdk8s-cli@2.203.18 \
    && npm cache clean --force

COPY --from=go_runtime_builder /usr/local/go /usr/local/go
COPY --from=builder /workspace/manager .
COPY --from=sshbuilder /ssh/ssh_known_hosts /etc/ssh/ssh_known_hosts

# Set Go environment variables
ENV PATH=$PATH:/usr/local/go/bin:/usr/local/bin
ENV GOROOT=/usr/local/go

# Create non-root user
RUN adduser -u 65532 -D -h /home/nonroot -s /bin/sh nonroot

USER 65532
ENTRYPOINT ["/manager"]

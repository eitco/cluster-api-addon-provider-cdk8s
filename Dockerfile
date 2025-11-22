# syntax=docker/dockerfile:0.4

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
ARG ARCH
ARG package=.
ARG ldflags

# Build step only to fetch the ssh_known_hosts
FROM ${deployment_base_image}:${deployment_base_image_tag} AS sshbuilder
ARG openssh_version
ARG openssh_client_version

WORKDIR /ssh

RUN apk add --no-cache openssh=${openssh_version} openssh-client=${openssh_client_version}

COPY ./hack/update-ssh-known-hosts.sh ./

# Known Hosts
RUN ./update-ssh-known-hosts.sh

# Ignore Hadolint rule "Always tag the version of an image explicitly."
# It's an invalid finding since the image is explicitly set in the Makefile.
# https://github.com/hadolint/hadolint/wiki/DL3006
# hadolint ignore=DL3006
FROM ${builder_image} AS builder
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

# Do not force rebuild of up-to-date packages (do not use -a) and use the compiler cache folder
# hadolint ignore=SC2086
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} \
    go build -trimpath -ldflags "${ldflags} -extldflags '-static'" \
    -o manager ${package}

# Go Runtime Builder
FROM ${deployment_base_image}:${deployment_base_image_tag} AS go_runtime_builder
ARG ARCH
ARG go_version
ARG curl_version
ARG xz_version
ARG tar_version

RUN apk add --no-cache curl=${curl_version} tar=${tar_version} xz=${xz_version}
RUN curl -fsSL -o go${go_version}.linux-${ARCH}.tar.gz https://go.dev/dl/go${go_version}.linux-${ARCH}.tar.gz \
    && tar -C /usr/local -xzf go${go_version}.linux-${ARCH}.tar.gz \
    && rm go${go_version}.linux-${ARCH}.tar.gz 

# NODE Runtime Builder
# FROM alpine:3.22.2 as node_runtime_builder

# RUN apk add --no-cache nodejs=22.16.0-r2 npm=11.4.2-r0  \
#     && npm install -g cdk8s-cli@2.202.3 \
#     && npm cache clean --force \
#     && rm -rf /root/.npm \
#     && rm -rf /var/cache/apk/*

# Production image
FROM ${deployment_base_image}:${deployment_base_image_tag}
ARG ARCH
ARG nodejs_version
ARG npm_version
ARG cdk8s_version

WORKDIR /

RUN apk add --no-cache nodejs=${nodejs_version} npm=${npm_version} \
    && npm install -g cdk8s-cli@${cdk8s_version} \
    && npm cache clean --force \
    && rm -rf /root/.npm /tmp/* /var/cache/apk/*

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

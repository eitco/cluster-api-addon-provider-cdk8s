# Quick Start Guide

https://cluster-api.sigs.k8s.io/user/quick-start

## Setup KinD Cluster
```
cat > kind-cluster-with-extramounts.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  ipFamily: dual
nodes:
- role: control-plane
  extraMounts:
    - hostPath: /var/run/docker.sock
      containerPath: /var/run/docker.sock
EOF
```

`kind create cluster --config kind-cluster-with-extramounts.yaml`

## Install Cluster-API Components to management-Cluster
```
# Enable the experimental Cluster topology feature.
export CLUSTER_TOPOLOGY=true

# Initialize the management cluster
clusterctl init --infrastructure docker --addon eitco-cdk8s
```

## Generate Workload-Cluster yaml

```
clusterctl generate cluster capi-quickstart --flavor development \
  --kubernetes-version v1.33.0 \
  --control-plane-machine-count=3 \
  --worker-machine-count=3 \
  > capi-quickstart.yaml
```

## Create Workload-Cluster

```
kubectl apply -f capi-quickstart.yaml
```

## Get Kubeconfig
```
clusterctl get kubeconfig capi-quickstart > capi-quickstart.kubeconfig
```

## Modify Kubeconfig (only for macOS)
```
sed -i -e "s/server:.*/server: https:\/\/$(docker port capi-quickstart-lb 6443/tcp | sed "s/0.0.0.0/127.0.0.1/")/g" ./capi-quickstart.kubeconfig
```

## Apply CNI calico

```
kubectl --kubeconfig=./capi-quickstart.kubeconfig \
  apply -f calico.yaml
```

for Downloadpath see CAPI Quickstart Guide

## Apply cdk8sAppProxy resource to management-Cluster

```
kubectl apply -f - << EOF
apiVersion: addons.cluster.x-k8s.io/v1alpha1
kind: Cdk8sAppProxy
metadata:
  name: cdk8s-sample-app-go
  namespace: default
spec:
  gitRepository:
    url: "https://github.com/eitco/cluster-api-addon-provider-cdk8s"
    reference: "main"
    referencePollInterval: "5m"
    path: "examples/cdk8s-sample-deployment"
  clusterSelector: {}
EOF
```

### more examples:
https://github.com/PatrickLaabs/cdk8s-sample-deployment/tree/main
https://github.com/PatrickLaabs/cdk8s-sample-deployment-typescript

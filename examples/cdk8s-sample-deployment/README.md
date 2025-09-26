# CDK8s Sample Deployment - Go

## Quick Start

This example demonstrates how to create and deploy a Kubernetes application using CDK8s in Go.

### Prerequisites
- Go 1.19+
- CDK8s CLI (`npm install -g cdk8s-cli`)
- kubectl
- Kubernetes cluster (minikube, kind, or Docker Desktop)

### Deploy in 3 Steps

```bash
# 1. Install dependencies
go mod tidy

# 2. Generate Kubernetes manifests
cdk8s synth

# 3. Deploy to your cluster
kubectl apply -f dist/
```

### Verify Deployment

```bash
# View deployments
kubectl get deployments

# View pods
kubectl get pods

# Access the application
kubectl port-forward deployment/getting-started-deployment-<hash> 8080:80
```

Open http://localhost:8080 in browser.

Check out [KUBERNETES_GUIDE.md](./KUBERNETES_GUIDE.md) for detailed explanations of the concepts used in this example.

## Project Structure

- `main.go` - CDK8s code that defines the application
- `cdk8s.yaml` - CDK8s configuration
- `dist/` - Generated Kubernetes manifests
- `imports/k8s/` - Generated Go types for Kubernetes

## Kubernetes Resources Created

- **Deployment**: `getting-started-deployment-<hash>`
    - 1 replica
    - Image: nginx:1.19.10
    - Port: 80
    - Labels: app=my-app

## Cleanup

Remove the application
```bash
kubectl delete -f dist/
```

# Kubernetes Guide - CDK8s Go Example

## What This Example Creates

### 1. Kubernetes Deployment

The Go code generates a **Deployment** that:
- Runs 1 instance of nginx
- Manages high availability
- Automatically restarts on failure

### 2. Pod Template

Each pod contains:
- **Container**: nginx:1.19.10
- **Port**: 80 (HTTP)
- **Labels**: app=my-app

## Understanding the Go Code

### NewChart Function
```go
func NewChart(scope constructs.Construct, id string, ns string, appLabel string) cdk8s.Chart
```
- Creates a CDK8s "chart" (group of resources)
- Parameters: scope, id, namespace, label

### Deployment Creation
```go
k8s.NewKubeDeployment(chart, jsii.String("deployment"), &k8s.KubeDeploymentProps{
    Spec: &k8s.DeploymentSpec{
        Replicas: jsii.Number(1),  // 1 instance
        // ... configuration ...
    },
})
```

## Complete Workflow

1. **Go Code** → Defines the application
2. **cdk8s synth** → Generates Kubernetes YAML
3. **kubectl apply** → Deploys to cluster
4. **Port-forward** → Local access

## Understanding the Generated YAML

### Deployment Spec
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: getting-started-deployment-c80c7257
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - image: nginx:1.19.10
          name: app-container
          ports:
            - containerPort: 80
```

## Troubleshooting

### Application won't start
```bash
kubectl describe pod <pod-name>
kubectl logs <pod-name>
```

### Port-forward not working
```bash
kubectl get pods
kubectl port-forward deployment/<deployment-name> 8080:80
```

### Manifest not generated
```bash
cdk8s synth
ls -la dist/
```

### Check deployment status
```bash
kubectl get deployments
kubectl describe deployment getting-started-deployment-<hash>
```
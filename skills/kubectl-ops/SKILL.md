---
name: kubectl-ops
description: "Apply manifests, manage rollouts, scale deployments, configure services, and manage secrets on Kubernetes clusters using kubectl. Trigger: when deploying to Kubernetes, using kubectl, applying manifests, managing K8s deployments, scaling Kubernetes workloads, Kubernetes rollouts, kubectl port-forward"
version: 1
argument-hint: "[apply|get|rollout|scale|exec|port-forward|secrets]"
allowed-tools:
  - bash
  - read
  - write
  - edit
  - glob
  - grep
---
# Kubernetes Operations

You are now operating in Kubernetes operations mode using `kubectl`.

## Prerequisites

```bash
# Install kubectl
# macOS
brew install kubectl

# Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl && sudo mv kubectl /usr/local/bin/

# Verify version
kubectl version --client

# Verify cluster connection
kubectl cluster-info
```

## Context and Namespace Management

```bash
# List all configured contexts (clusters)
kubectl config get-contexts

# Switch to a context
kubectl config use-context my-cluster

# Show current context
kubectl config current-context

# Set the default namespace for the current context
kubectl config set-context --current --namespace=my-namespace

# View full kubeconfig
kubectl config view

# Run a command in a specific namespace (override default)
kubectl get pods -n my-namespace

# Run a command against a specific context
kubectl get pods --context=my-other-cluster
```

## Applying Manifests

```bash
# Apply a single manifest file
kubectl apply -f deployment.yaml

# Apply all manifests in a directory
kubectl apply -f ./manifests/

# Apply manifests recursively
kubectl apply -f ./k8s/ -R

# Apply from stdin
cat deployment.yaml | kubectl apply -f -

# Apply and wait for rollout to complete
kubectl apply -f deployment.yaml
kubectl rollout status deployment/my-app -n my-namespace

# Dry-run to preview changes (no actual apply)
kubectl apply -f deployment.yaml --dry-run=client

# Show diff before applying
kubectl diff -f deployment.yaml
```

## Viewing Resources

```bash
# List pods in current namespace
kubectl get pods

# List pods in a specific namespace
kubectl get pods -n my-namespace

# List pods in JSON format (for parsing)
kubectl get pods -n my-namespace -o json

# List pods with node assignment and IP
kubectl get pods -n my-namespace -o wide

# Watch pods in real-time
kubectl get pods -n my-namespace --watch

# List all resources of a type with labels
kubectl get pods -l app=my-app -n my-namespace

# List deployments
kubectl get deployments -n my-namespace

# List services
kubectl get services -n my-namespace

# List all resource types at once
kubectl get all -n my-namespace

# List across all namespaces
kubectl get pods -A
kubectl get deployments -A

# Get a specific resource by name
kubectl get pod my-pod-xyz -n my-namespace -o yaml

# List persistent volume claims
kubectl get pvc -n my-namespace
```

## Describing Resources (Detailed View)

```bash
# Describe a pod (shows events, conditions, mounts)
kubectl describe pod <pod-name> -n my-namespace

# Describe a deployment
kubectl describe deployment my-app -n my-namespace

# Describe a service
kubectl describe service my-svc -n my-namespace

# Describe a node
kubectl describe node <node-name>
```

## Rollout Management

```bash
# Watch a rollout in progress
kubectl rollout status deployment/my-app -n my-namespace

# View rollout history
kubectl rollout history deployment/my-app -n my-namespace

# View a specific revision's details
kubectl rollout history deployment/my-app -n my-namespace --revision=2

# Rollback to the previous version
kubectl rollout undo deployment/my-app -n my-namespace

# Rollback to a specific revision
kubectl rollout undo deployment/my-app -n my-namespace --to-revision=2

# Pause a rolling update
kubectl rollout pause deployment/my-app -n my-namespace

# Resume a paused rollout
kubectl rollout resume deployment/my-app -n my-namespace

# Restart all pods in a deployment (rolling restart)
kubectl rollout restart deployment/my-app -n my-namespace
```

## Scaling

```bash
# Scale a deployment to N replicas
kubectl scale deployment/my-app --replicas=3 -n my-namespace

# Scale a StatefulSet
kubectl scale statefulset/my-db --replicas=3 -n my-namespace

# Scale and wait for the operation to complete
kubectl scale deployment/my-app --replicas=3 -n my-namespace
kubectl rollout status deployment/my-app -n my-namespace

# Auto-scale based on CPU utilization (HPA)
kubectl autoscale deployment/my-app --min=2 --max=10 --cpu-percent=80 -n my-namespace

# List HPAs
kubectl get hpa -n my-namespace
```

## Updating Images

```bash
# Update a container image in a deployment
kubectl set image deployment/my-app \
  my-container=myregistry/my-app:v2.0.0 \
  -n my-namespace

# Update and watch the rollout
kubectl set image deployment/my-app \
  my-container=myregistry/my-app:v2.0.0 \
  -n my-namespace && \
kubectl rollout status deployment/my-app -n my-namespace
```

## Viewing Logs

```bash
# View logs from a pod
kubectl logs <pod-name> -n my-namespace

# Follow logs in real-time
kubectl logs <pod-name> -n my-namespace -f

# View logs from a specific container in a multi-container pod
kubectl logs <pod-name> -c my-container -n my-namespace

# View logs from the previously crashed container
kubectl logs <pod-name> --previous -n my-namespace

# View logs from all pods with a label selector
kubectl logs -l app=my-app -n my-namespace --prefix

# Tail the last N lines
kubectl logs <pod-name> -n my-namespace --tail=100

# View logs since a duration
kubectl logs <pod-name> -n my-namespace --since=1h
```

## Executing Commands in Pods

```bash
# Open an interactive shell in a pod
kubectl exec -it <pod-name> -n my-namespace -- sh

# Open bash if available
kubectl exec -it <pod-name> -n my-namespace -- bash

# Run a one-off command (non-interactive)
kubectl exec <pod-name> -n my-namespace -- ls /app

# Exec in a specific container within a multi-container pod
kubectl exec -it <pod-name> -c my-container -n my-namespace -- sh

# Check environment variables in a pod
kubectl exec <pod-name> -n my-namespace -- env | sort

# Test connectivity from inside a pod
kubectl exec <pod-name> -n my-namespace -- curl -s http://other-service/health
```

## Port Forwarding

```bash
# Forward a local port to a pod
kubectl port-forward pod/<pod-name> 8080:80 -n my-namespace

# Forward a local port to a service (load-balanced)
kubectl port-forward svc/my-service 8080:80 -n my-namespace

# Forward a local port to a deployment
kubectl port-forward deployment/my-app 8080:80 -n my-namespace

# Forward on all interfaces (accessible from other machines)
kubectl port-forward svc/my-service 8080:80 -n my-namespace --address 0.0.0.0

# Run in background (use with care — press Ctrl+C to stop)
kubectl port-forward svc/my-service 8080:80 -n my-namespace &
PORT_FORWARD_PID=$!
# ... do work ...
kill $PORT_FORWARD_PID
```

## Secrets Management

```bash
# Create a generic secret from literal values
kubectl create secret generic my-secret \
  --from-literal=DATABASE_URL="postgresql://..." \
  --from-literal=API_KEY="sk_live_..." \
  -n my-namespace

# Create a secret from a file
kubectl create secret generic my-secret \
  --from-file=tls.crt=./cert.pem \
  --from-file=tls.key=./key.pem \
  -n my-namespace

# Create a TLS secret
kubectl create secret tls my-tls-secret \
  --cert=./tls.crt \
  --key=./tls.key \
  -n my-namespace

# Create a Docker registry secret
kubectl create secret docker-registry regcred \
  --docker-server=registry.example.com \
  --docker-username=myuser \
  --docker-password=mypass \
  -n my-namespace

# List secret names (never print values)
kubectl get secrets -n my-namespace

# View secret metadata without decoding values
kubectl describe secret my-secret -n my-namespace

# SAFE: decode a specific key for verification only
kubectl get secret my-secret -n my-namespace \
  -o jsonpath='{.data.DATABASE_URL}' | base64 --decode

# Update a secret (apply new manifest)
kubectl apply -f secret.yaml

# Delete a secret
kubectl delete secret my-secret -n my-namespace
```

## ConfigMaps

```bash
# Create a ConfigMap from literal values
kubectl create configmap my-config \
  --from-literal=LOG_LEVEL=info \
  --from-literal=MAX_CONNECTIONS=100 \
  -n my-namespace

# Create a ConfigMap from a file
kubectl create configmap app-config \
  --from-file=config.yaml \
  -n my-namespace

# View ConfigMap contents
kubectl get configmap my-config -n my-namespace -o yaml

# Update a ConfigMap
kubectl edit configmap my-config -n my-namespace
```

## Resource Monitoring

```bash
# Resource usage for all pods in a namespace
kubectl top pods -n my-namespace

# Resource usage for all nodes
kubectl top nodes

# Resource usage sorted by CPU
kubectl top pods -n my-namespace --sort-by=cpu

# Resource usage sorted by memory
kubectl top pods -n my-namespace --sort-by=memory

# Note: kubectl top requires metrics-server to be installed in the cluster
```

## Deleting Resources

```bash
# Delete a specific pod (will be recreated by the deployment controller)
kubectl delete pod <pod-name> -n my-namespace

# Delete a deployment (removes all pods managed by it)
kubectl delete deployment my-app -n my-namespace

# Delete using a manifest file
kubectl delete -f deployment.yaml

# Delete all resources matching a label
kubectl delete pods -l app=my-app -n my-namespace

# Delete a namespace (deletes all resources within it)
kubectl delete namespace my-namespace
```

## Common Manifest Examples

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-namespace
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: my-app
        image: myregistry/my-app:v1.0.0
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: my-secret
              key: DATABASE_URL
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
```

### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
spec:
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

## Safety Rules

- Always use `kubectl diff` before `kubectl apply` in production to preview changes.
- Never print secret values to logs or stdout. Use `kubectl describe secret` to view metadata only.
- Use `kubectl rollout undo` immediately if a deployment causes errors after apply.
- Set resource `requests` and `limits` on all containers to prevent resource exhaustion.
- Test manifest changes with `--dry-run=client` before applying to production.
- Use namespaces to isolate environments (dev, staging, production) within a cluster.
- Prefer `kubectl apply` over `kubectl create` for idempotent operations.

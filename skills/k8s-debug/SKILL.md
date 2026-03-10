---
name: k8s-debug
description: "Debug Kubernetes issues: CrashLoopBackOff, OOMKilled, ImagePullBackOff, pending pods, networking, and resource limits using kubectl. Trigger: when debugging Kubernetes pods, troubleshooting CrashLoopBackOff, OOMKilled pods, ImagePullBackOff errors, pending Kubernetes pods, K8s networking issues"
version: 1
argument-hint: "[pod-name or issue: crashloop|oom|imagepull|pending|networking]"
allowed-tools:
  - bash
  - read
  - grep
---
# Kubernetes Debugging

You are now operating in Kubernetes debugging mode.

## Standard Debug Workflow

When a pod is not working, follow these steps in order:

```bash
# Step 1: Identify the problem pod
kubectl get pods -n <namespace>
# Look for: CrashLoopBackOff, Error, Pending, OOMKilled, ImagePullBackOff

# Step 2: Get detailed pod status and events
kubectl describe pod <pod-name> -n <namespace>
# Key sections: Conditions, Events, Last State

# Step 3: View current container logs
kubectl logs <pod-name> -n <namespace>

# Step 4: View logs from previous (crashed) container
kubectl logs <pod-name> -n <namespace> --previous

# Step 5: If the container is running, exec in for inspection
kubectl exec -it <pod-name> -n <namespace> -- sh

# Step 6: Check resource usage
kubectl top pod <pod-name> -n <namespace>

# Step 7: Check cluster-level events
kubectl get events -n <namespace> --sort-by='.lastTimestamp'
```

## CrashLoopBackOff

The container starts, crashes, and Kubernetes keeps restarting it.

```bash
# Check the current container logs
kubectl logs <pod-name> -n <namespace>

# MOST IMPORTANT: Check logs from the previous (crashed) container
kubectl logs <pod-name> -n <namespace> --previous

# Get the exit code and reason
kubectl describe pod <pod-name> -n <namespace> | grep -A5 "Last State:"
# Look for: Exit Code, Reason (e.g., Error, OOMKilled)

# Check the last restart reason in JSON
kubectl get pod <pod-name> -n <namespace> -o json | \
  jq '.status.containerStatuses[0] | {restartCount, lastState}'

# Common causes and fixes:
# - Exit code 1: Application error — check app logs above
# - Exit code 137: OOMKilled — increase memory limits (see OOMKilled section)
# - Exit code 126/127: Command not found or not executable — check container entrypoint
# - Exit code 2: Misuse of shell builtins — check startup script
```

**Common Causes:**
- Application crashes on startup (missing env var, bad config, connection failure)
- Missing required secrets or ConfigMaps
- Database connection failures
- Incorrect entrypoint command in Dockerfile or pod spec

**Resolution Steps:**
1. Read the `--previous` logs first — they show why the container crashed
2. Check environment variables are present: `kubectl exec ... -- env | grep MY_VAR`
3. Verify secrets/configmaps exist: `kubectl get secret my-secret -n <namespace>`
4. Test the database connection separately using port-forward

## OOMKilled (Out of Memory)

The Linux OOM killer terminated the container because it exceeded its memory limit.

```bash
# Check if a pod was OOMKilled
kubectl describe pod <pod-name> -n <namespace> | grep -A3 "Last State:"
# Look for: Reason: OOMKilled

# Get current resource limits and usage
kubectl describe pod <pod-name> -n <namespace> | grep -A6 "Limits:"

# Check current memory usage (requires metrics-server)
kubectl top pod <pod-name> -n <namespace>

# Check all pods for OOMKilled status
kubectl get pods -n <namespace> -o json | \
  jq '.items[] | select(.status.containerStatuses[0].lastState.terminated.reason == "OOMKilled") | .metadata.name'

# View memory usage across all pods
kubectl top pods -n <namespace> --sort-by=memory
```

**Resolution:**

```yaml
# Increase memory limits in the deployment manifest
resources:
  requests:
    memory: 256Mi    # minimum guaranteed
  limits:
    memory: 512Mi    # maximum allowed (increase this)
```

```bash
# Apply the updated limits
kubectl apply -f deployment.yaml
kubectl rollout status deployment/my-app -n <namespace>
```

**If memory is legitimately high:**
- Profile the application for memory leaks
- Check for unbounded caches or connection pools
- Consider using a larger node or VM class

## ImagePullBackOff / ErrImagePull

Kubernetes cannot pull the container image.

```bash
# Get the full error message
kubectl describe pod <pod-name> -n <namespace> | grep -A10 "Events:"
# Look for: Failed to pull image, unauthorized, not found

# Common error patterns:
# "not found" -> wrong image name or tag
# "unauthorized" -> missing or invalid registry credentials
# "connection refused" -> registry is unreachable

# Check the image name in the pod spec
kubectl get pod <pod-name> -n <namespace> -o jsonpath='{.spec.containers[0].image}'

# Check if the image tag exists in the registry
docker manifest inspect myregistry/my-app:v2.0.0 2>&1
```

**Resolution — Wrong Image Name/Tag:**

```bash
# Fix the image tag in the deployment
kubectl set image deployment/my-app \
  my-container=myregistry/my-app:v1.0.0 \  # correct tag
  -n <namespace>
```

**Resolution — Missing Registry Credentials:**

```bash
# Create a Docker registry secret
kubectl create secret docker-registry regcred \
  --docker-server=registry.example.com \
  --docker-username=myuser \
  --docker-password=mypass \
  -n <namespace>

# Add imagePullSecrets to the deployment manifest
# spec:
#   imagePullSecrets:
#   - name: regcred

kubectl apply -f deployment.yaml
```

**Resolution — Private Registry (ECR, GCR, ACR):**

```bash
# AWS ECR: refresh credentials (they expire every 12h)
aws ecr get-login-password --region us-east-1 | \
  kubectl create secret docker-registry ecr-cred \
  --docker-server=<account>.dkr.ecr.us-east-1.amazonaws.com \
  --docker-username=AWS \
  --docker-password-stdin \
  -n <namespace>
```

## Pending Pods

Pods remain in `Pending` state, meaning Kubernetes cannot schedule them.

```bash
# Get the reason a pod is pending
kubectl describe pod <pod-name> -n <namespace> | grep -A20 "Events:"

# Common reasons:
# "0/3 nodes are available: Insufficient cpu" -> not enough CPU
# "0/3 nodes are available: Insufficient memory" -> not enough memory
# "0/3 nodes are available: node(s) didn't match Pod's node affinity" -> affinity mismatch
# "0/1 nodes are available: pod has unbound immediate PersistentVolumeClaims" -> PVC issue

# Check node capacity and allocatable resources
kubectl describe nodes | grep -A5 "Allocatable:"

# Check how much is already allocated
kubectl describe nodes | grep -A8 "Allocated resources:"

# Check PVC status if pending due to storage
kubectl get pvc -n <namespace>
kubectl describe pvc <pvc-name> -n <namespace>
```

**Resolution — Insufficient Resources:**

```yaml
# Reduce resource requests in the deployment manifest
resources:
  requests:
    cpu: 50m       # reduce from 500m
    memory: 64Mi   # reduce from 512Mi
```

**Resolution — Node Affinity Mismatch:**

```bash
# Check node labels
kubectl get nodes --show-labels

# Remove or relax the affinity rule in the deployment manifest
# spec:
#   affinity:
#     nodeAffinity:  <-- remove this if not needed
```

**Resolution — PVC Not Bound:**

```bash
# Check if a matching PersistentVolume exists
kubectl get pv

# Check storage class availability
kubectl get storageclass

# If using dynamic provisioning, verify the storage class is installed
kubectl describe pvc <pvc-name> -n <namespace>
```

## Networking Issues

```bash
# Test DNS resolution from inside a pod
kubectl exec <pod-name> -n <namespace> -- nslookup other-service
kubectl exec <pod-name> -n <namespace> -- nslookup other-service.other-namespace.svc.cluster.local

# Test connectivity to a service by DNS name
kubectl exec <pod-name> -n <namespace> -- curl -s http://my-service/health

# Test connectivity to a service by ClusterIP
SVC_IP=$(kubectl get svc my-service -n <namespace> -o jsonpath='{.spec.clusterIP}')
kubectl exec <pod-name> -n <namespace> -- curl -s http://$SVC_IP/health

# Check that a service has endpoints (pods selected)
kubectl get endpoints my-service -n <namespace>
# If ENDPOINTS is "<none>", the service selector doesn't match any pods

# Check service selector matches pod labels
kubectl describe svc my-service -n <namespace> | grep Selector:
kubectl get pods -n <namespace> --show-labels

# Check NetworkPolicies that may be blocking traffic
kubectl get networkpolicies -n <namespace>
kubectl describe networkpolicy <policy-name> -n <namespace>

# Run a debug pod with networking tools
kubectl run debug-pod --image=nicolaka/netshoot --rm -it --restart=Never -n <namespace> -- sh
# Inside: ping, curl, nslookup, traceroute, tcpdump are all available
```

## Config and Environment Issues

```bash
# Check all environment variables in a running pod
kubectl exec <pod-name> -n <namespace> -- env | sort

# Check if a specific environment variable is set
kubectl exec <pod-name> -n <namespace> -- printenv MY_VAR

# Verify a secret exists and has the right keys
kubectl describe secret my-secret -n <namespace>
# Shows keys but NOT values (by design)

# Verify a ConfigMap and its contents
kubectl get configmap my-config -n <namespace> -o yaml

# Check if the pod mounts the correct secret/configmap
kubectl describe pod <pod-name> -n <namespace> | grep -A10 "Mounts:"
kubectl describe pod <pod-name> -n <namespace> | grep -A10 "Volumes:"
```

## Ephemeral Debug Containers

For pods without a shell or debugging tools:

```bash
# Attach an ephemeral debug container to a running pod
kubectl debug -it <pod-name> -n <namespace> \
  --image=busybox \
  --target=my-container

# Use a richer debug image
kubectl debug -it <pod-name> -n <namespace> \
  --image=nicolaka/netshoot \
  --target=my-container

# Copy a pod and add a debug shell (useful for initContainer issues)
kubectl debug <pod-name> -n <namespace> \
  --copy-to=debug-pod \
  --container=my-container \
  --image=busybox \
  -it -- sh
```

## Cluster-Level Diagnostics

```bash
# View all events across the cluster, sorted by time
kubectl get events -A --sort-by='.lastTimestamp' | tail -30

# View events for a specific namespace
kubectl get events -n <namespace> --sort-by='.lastTimestamp'

# Filter events by reason
kubectl get events -n <namespace> --field-selector reason=OOMKilling
kubectl get events -n <namespace> --field-selector reason=BackOff

# Check node conditions
kubectl get nodes
# Look for: Ready, MemoryPressure, DiskPressure, PIDPressure

# Describe a specific node
kubectl describe node <node-name>

# Check pod disruption budgets
kubectl get pdb -n <namespace>
```

## Quick Diagnostic Summary

```bash
# One-liner to show problem pods across all namespaces
kubectl get pods -A | grep -v Running | grep -v Completed | grep -v Terminating

# Get restart counts for all pods
kubectl get pods -n <namespace> -o json | \
  jq '.items[] | {name: .metadata.name, restarts: .status.containerStatuses[0].restartCount}' | \
  jq -s 'sort_by(.restarts) | reverse[]'

# Show most recently failed events
kubectl get events -n <namespace> \
  --field-selector type=Warning \
  --sort-by='.lastTimestamp' | tail -20
```

## Safety Rules

- Never expose secret values in logs or `kubectl exec` output. Use `kubectl describe secret` to check key names only.
- Use `kubectl exec` with caution in production — commands run directly on live containers.
- Avoid `kubectl delete pod` in production unless you intend to force a restart (the deployment will recreate it).
- `kubectl debug --copy-to` creates a new pod; clean it up with `kubectl delete pod debug-pod` when done.
- Resource limit changes require a rolling restart — use `kubectl rollout status` to verify the restart completes.
- `kubectl delete namespace` is irreversible and removes all resources — confirm the namespace name before running.

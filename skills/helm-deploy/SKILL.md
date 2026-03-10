---
name: helm-deploy
description: "Deploy and manage Kubernetes applications with Helm: install, upgrade, rollback, chart management, values.yaml, secrets. Trigger: when deploying with Helm, helm install, helm upgrade, helm rollback, Helm charts, Kubernetes package management, helm values, chart templates"
version: 1
argument-hint: "[install|upgrade|rollback|list|uninstall|template <release> <chart>]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Helm — Kubernetes Package Management

You are now operating in Helm deployment management mode.

## Installation

```bash
# macOS
brew install helm

# Linux
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Verify
helm version
```

## Repository Management

```bash
# Add a chart repository
helm repo add stable https://charts.helm.sh/stable
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx

# Update repository index
helm repo update

# List configured repositories
helm repo list

# Remove a repository
helm repo remove stable

# Search for charts in configured repos
helm search repo nginx
helm search repo bitnami/postgresql --versions

# Search Artifact Hub (public registry)
helm search hub wordpress
```

## Installing Charts

```bash
# Install a chart with default values
helm install my-release bitnami/postgresql

# Install with custom values file
helm install my-release bitnami/postgresql -f values.yaml

# Install with inline value overrides
helm install my-release bitnami/postgresql \
  --set auth.postgresPassword=secret \
  --set primary.persistence.size=10Gi

# Install into a specific namespace (create if missing)
helm install my-release bitnami/postgresql \
  --namespace production \
  --create-namespace

# Install with a specific chart version
helm install my-release bitnami/postgresql --version 12.1.3

# Install from a local chart directory
helm install my-release ./my-chart/

# Install from a local packaged chart
helm install my-release ./my-chart-1.0.0.tgz

# Dry run (preview what would be installed)
helm install my-release bitnami/postgresql --dry-run --debug
```

## Upgrading Releases

```bash
# Upgrade an existing release
helm upgrade my-release bitnami/postgresql -f values.yaml

# Install if not present, upgrade if it is (idempotent)
helm upgrade --install my-release bitnami/postgresql -f values.yaml

# Upgrade with timeout and wait for rollout
helm upgrade --install my-release bitnami/postgresql \
  -f values.yaml \
  --wait \
  --timeout 5m

# Upgrade and clean up old resources no longer in chart
helm upgrade my-release bitnami/postgresql \
  -f values.yaml \
  --cleanup-on-fail

# Atomic upgrade (rollback on failure automatically)
helm upgrade --atomic my-release bitnami/postgresql -f values.yaml

# Upgrade with set values
helm upgrade my-release bitnami/postgresql \
  --reuse-values \
  --set image.tag=15.3.0
```

## Rollback

```bash
# List revision history for a release
helm history my-release

# Rollback to the previous revision
helm rollback my-release

# Rollback to a specific revision number
helm rollback my-release 2

# Rollback with wait
helm rollback my-release 2 --wait --timeout 3m

# Rollback in a specific namespace
helm rollback my-release 1 --namespace production
```

## Release Management

```bash
# List all releases in current namespace
helm list

# List releases in all namespaces
helm list --all-namespaces

# List releases in a specific namespace
helm list --namespace production

# List failed releases
helm list --failed

# Show release status (pods, services, etc.)
helm status my-release

# Show deployed values for a release
helm get values my-release

# Show all computed values (including defaults)
helm get values my-release --all

# Show the manifests that were applied
helm get manifest my-release

# Show release notes
helm get notes my-release

# Uninstall a release (DESTRUCTIVE)
helm uninstall my-release
# Note: This deletes all Kubernetes resources created by the chart.
# Always confirm the release name and namespace before uninstalling.

# Uninstall but keep history
helm uninstall my-release --keep-history

# Uninstall from a specific namespace
helm uninstall my-release --namespace production
```

## Chart Development

```bash
# Create a new chart scaffold
helm create my-chart

# Validate chart structure and templates
helm lint my-chart/

# Lint with custom values
helm lint my-chart/ -f custom-values.yaml

# Render templates locally without installing (debugging)
helm template my-release my-chart/ -f values.yaml

# Render and apply to cluster (like kubectl apply)
helm template my-release my-chart/ | kubectl apply -f -

# Package a chart into a .tgz archive
helm package my-chart/

# Package with version override
helm package my-chart/ --version 1.2.3 --app-version 2.0.0
```

## values.yaml Patterns

```yaml
# values.yaml — default values for chart
replicaCount: 2

image:
  repository: myapp
  tag: "1.0.0"
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 80

ingress:
  enabled: false
  className: nginx
  hosts:
    - host: myapp.example.com
      paths:
        - path: /
          pathType: Prefix

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80

env:
  - name: APP_ENV
    value: production
```

## Environment-Specific Values

```bash
# Use multiple values files (later files override earlier)
helm upgrade --install my-release ./my-chart \
  -f values.yaml \
  -f values-production.yaml \
  --set image.tag=git-$(git rev-parse --short HEAD)

# Structure for multiple environments:
# values.yaml           — base defaults
# values-staging.yaml   — staging overrides
# values-production.yaml — production overrides
```

## Secrets Management

```bash
# NEVER store secrets in plain values.yaml in git.

# Option 1: Pass secrets at deploy time via --set
helm upgrade --install my-release ./my-chart \
  --set db.password="${DB_PASSWORD}"

# Option 2: Use Helm Secrets plugin (encrypts with SOPS/GPG)
helm plugin install https://github.com/jkroepke/helm-secrets
helm secrets upgrade --install my-release ./my-chart \
  -f secrets.yaml.enc

# Option 3: Reference existing Kubernetes Secret
# In values.yaml:
# existingSecret: my-app-secret
# The chart template then uses: valueFrom.secretKeyRef

# Option 4: Use ExternalSecrets Operator or Vault Agent Injector
# (store secrets in external vault, sync to K8s Secrets)
```

## Troubleshooting

```bash
# Debug a failed install — show rendered manifests and error
helm install my-release ./my-chart --debug --dry-run 2>&1

# Check events for a failed deployment
kubectl get events --sort-by='.lastTimestamp' -n production | tail -20

# Describe a stuck pod
kubectl describe pod <pod-name> -n production

# Get logs from a crashlooping container
kubectl logs <pod-name> -n production --previous

# Check if chart values are correct
helm get values my-release --all | grep <key>

# Diff current release vs proposed upgrade (requires helm-diff plugin)
helm plugin install https://github.com/databus23/helm-diff
helm diff upgrade my-release ./my-chart -f values.yaml
```

## Best Practices

- Always use `helm upgrade --install` for idempotent deployments.
- Use `--atomic` in CI to auto-rollback on deployment failure.
- Never commit plaintext secrets in `values.yaml`; use `--set` or Helm Secrets plugin.
- Pin chart versions in production: `helm upgrade my-release bitnami/postgresql --version 12.1.3`.
- Use `helm lint` and `helm template` in CI to catch errors before deploying.
- Keep a `values-<env>.yaml` per environment; never modify base `values.yaml` for env-specific config.
- Run `helm list --all-namespaces` regularly to audit what is deployed.
- Use `helm diff upgrade` (with helm-diff plugin) to review changes before applying.

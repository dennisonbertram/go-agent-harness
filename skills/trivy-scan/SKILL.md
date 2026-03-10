---
name: trivy-scan
description: "Scan containers and filesystems for vulnerabilities with Trivy: trivy image, trivy fs, trivy repo, severity filtering, SARIF output, CI integration. Trigger: when using Trivy, trivy image, trivy fs, trivy scan, container vulnerability scan, trivy repo, CVE scanning, SBOM generation"
version: 1
argument-hint: "[image|fs|repo|config|sbom] [target] [--severity HIGH,CRITICAL]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Trivy Security Scanner

You are now operating in Trivy vulnerability scanning mode.

## Installation

```bash
# macOS (via Homebrew)
brew install trivy

# Linux (Debian/Ubuntu)
sudo apt-get install wget apt-transport-https gnupg lsb-release
wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | sudo apt-key add -
echo "deb https://aquasecurity.github.io/trivy-repo/deb $(lsb_release -sc) main" | sudo tee -a /etc/apt/sources.list.d/trivy.list
sudo apt-get update && sudo apt-get install trivy

# Using Docker (no installation required)
docker run --rm aquasec/trivy:latest image alpine:3.18

# Verify installation
trivy --version

# Update vulnerability database
trivy image --download-db-only
```

## Container Image Scanning

```bash
# Scan a Docker image
trivy image nginx:latest

# Scan a local image (built but not pushed)
trivy image myapp:local

# Scan with severity filtering
trivy image --severity HIGH,CRITICAL nginx:latest

# Severity levels: UNKNOWN, LOW, MEDIUM, HIGH, CRITICAL
trivy image --severity CRITICAL nginx:latest

# Scan and output as JSON
trivy image --format json nginx:latest

# Scan and output as SARIF (for GitHub Code Scanning)
trivy image --format sarif --output trivy-results.sarif nginx:latest

# Scan and output as table (default)
trivy image --format table nginx:latest

# Scan without showing unfixed vulnerabilities
trivy image --ignore-unfixed nginx:latest

# Combine: only show HIGH/CRITICAL that have fixes
trivy image --severity HIGH,CRITICAL --ignore-unfixed nginx:latest

# Scan a remote image from a private registry
trivy image --username user --password pass myregistry.com/myapp:latest

# Scan a tar archive
docker save myapp:latest -o myapp.tar
trivy image --input myapp.tar

# Exit with non-zero code if vulnerabilities found (for CI)
trivy image --exit-code 1 --severity HIGH,CRITICAL nginx:latest
```

## Filesystem Scanning

```bash
# Scan the current directory
trivy fs .

# Scan a specific directory
trivy fs /path/to/project

# Scan with severity filter
trivy fs --severity HIGH,CRITICAL .

# Scan and output as JSON
trivy fs --format json . > trivy-fs-results.json

# Scan package files only (no code analysis)
trivy fs --scanners vuln .

# Include secret detection
trivy fs --scanners vuln,secret .

# Include misconfig detection
trivy fs --scanners vuln,secret,misconfig .

# Scan and exit non-zero on findings
trivy fs --exit-code 1 --severity HIGH,CRITICAL .
```

## Repository Scanning

```bash
# Scan a remote Git repository
trivy repo https://github.com/myorg/myrepo

# Scan with authentication (private repos)
trivy repo --token $GITHUB_TOKEN https://github.com/myorg/private-repo

# Scan a local repository
trivy repo .

# Scan specific branch
trivy repo --branch main https://github.com/myorg/myrepo

# Scan specific commit
trivy repo --commit abc1234 https://github.com/myorg/myrepo
```

## Configuration/IaC Scanning

```bash
# Scan Terraform files for misconfigurations
trivy config ./terraform/

# Scan Kubernetes manifests
trivy config ./k8s/

# Scan Dockerfile
trivy config ./Dockerfile

# Scan Helm charts
trivy config ./charts/

# Scan CloudFormation templates
trivy config ./cloudformation/

# Scan with severity filtering
trivy config --severity HIGH,CRITICAL ./terraform/

# Show all checks including passed ones
trivy config --show-suppressed ./terraform/
```

## SBOM (Software Bill of Materials)

```bash
# Generate SBOM in CycloneDX format
trivy image --format cyclonedx --output sbom.json nginx:latest

# Generate SBOM in SPDX format
trivy image --format spdx-json --output sbom.spdx.json nginx:latest

# Generate SBOM for filesystem
trivy fs --format cyclonedx --output sbom.json .

# Scan an existing SBOM for vulnerabilities
trivy sbom ./sbom.json
```

## Severity Filtering and Thresholds

```bash
# Only report HIGH and CRITICAL vulnerabilities
trivy image --severity HIGH,CRITICAL myapp:latest

# Fail CI build if CRITICAL vulnerabilities found
trivy image --exit-code 1 --severity CRITICAL myapp:latest

# Fail CI build on any HIGH or CRITICAL
trivy image --exit-code 1 --severity HIGH,CRITICAL myapp:latest

# Show vulnerability counts by severity
trivy image --format json myapp:latest | \
  jq '.Results[].Vulnerabilities | group_by(.Severity) | map({severity: .[0].Severity, count: length})'

# Ignore specific CVEs
echo "CVE-2023-1234" > .trivyignore
trivy image myapp:latest

# .trivyignore format
cat > .trivyignore <<'EOF'
# Ignore CVEs with comments
CVE-2023-1234
CVE-2023-5678  # False positive confirmed
EOF
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/trivy.yml
name: Trivy Security Scan

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  trivy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build Docker image
        run: docker build -t myapp:${{ github.sha }} .

      - name: Run Trivy container scan
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: myapp:${{ github.sha }}
          format: sarif
          output: trivy-container.sarif
          severity: HIGH,CRITICAL
          exit-code: 1
          ignore-unfixed: true

      - name: Run Trivy filesystem scan
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: fs
          scan-ref: .
          format: sarif
          output: trivy-fs.sarif
          severity: HIGH,CRITICAL

      - name: Upload container SARIF to GitHub Security
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: trivy-container.sarif

      - name: Upload filesystem SARIF to GitHub Security
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: trivy-fs.sarif
```

### Shell Script for CI Pipeline

```bash
#!/bin/bash
set -euo pipefail

IMAGE="myapp:${CI_COMMIT_SHA:-latest}"
SEVERITY="HIGH,CRITICAL"
EXIT_CODE=0

echo "=== Scanning container image: $IMAGE ==="
trivy image \
  --severity "$SEVERITY" \
  --ignore-unfixed \
  --exit-code 1 \
  --format sarif \
  --output trivy-image.sarif \
  "$IMAGE" || EXIT_CODE=$?

echo "=== Scanning filesystem ==="
trivy fs \
  --severity "$SEVERITY" \
  --scanners vuln,secret \
  --exit-code 1 \
  --format sarif \
  --output trivy-fs.sarif \
  . || EXIT_CODE=$?

echo "=== Scanning IaC configurations ==="
trivy config \
  --severity "$SEVERITY" \
  --exit-code 1 \
  ./terraform/ ./k8s/ || EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
  echo "Trivy found vulnerabilities at severity $SEVERITY"
  exit $EXIT_CODE
fi

echo "All Trivy scans passed"
```

### Makefile Integration

```makefile
TRIVY_SEVERITY ?= HIGH,CRITICAL
IMAGE_NAME ?= myapp:latest

.PHONY: scan-image scan-fs scan-config scan-all
scan-image:
	trivy image --severity $(TRIVY_SEVERITY) --ignore-unfixed $(IMAGE_NAME)

scan-fs:
	trivy fs --severity $(TRIVY_SEVERITY) --scanners vuln,secret .

scan-config:
	trivy config --severity $(TRIVY_SEVERITY) ./terraform/ ./k8s/

scan-all: scan-image scan-fs scan-config
```

## Interpreting Results

```bash
# Parse JSON to find all CRITICAL vulnerabilities
trivy image --format json myapp:latest | \
  jq '.Results[].Vulnerabilities[] | select(.Severity == "CRITICAL") | {
    VulnerabilityID: .VulnerabilityID,
    PkgName: .PkgName,
    InstalledVersion: .InstalledVersion,
    FixedVersion: .FixedVersion,
    Title: .Title
  }'

# Count vulnerabilities by severity across all results
trivy image --format json myapp:latest | \
  jq '[.Results[].Vulnerabilities[]?.Severity] | group_by(.) | map({severity: .[0], count: length})'

# Find vulnerabilities with available fixes
trivy image --format json myapp:latest | \
  jq '.Results[].Vulnerabilities[] | select(.FixedVersion != "") | {
    id: .VulnerabilityID,
    pkg: .PkgName,
    installed: .InstalledVersion,
    fixedIn: .FixedVersion
  }'
```

## Caching and Performance

```bash
# Cache directory (default: ~/.cache/trivy)
# Set custom cache location
TRIVY_CACHE_DIR=/tmp/trivy-cache trivy image myapp:latest

# Download vulnerability database without scanning
trivy image --download-db-only

# Skip database update (use cached data)
trivy image --skip-db-update myapp:latest

# Use offline mode (no internet access)
trivy image --offline-scan myapp:latest

# Clear cache
trivy image --clear-cache
```

## Troubleshooting

```bash
# Enable debug logging
trivy image --debug myapp:latest

# Check database update time
trivy image --format json myapp:latest | jq '.SchemaVersion, .CreatedAt'

# Scan without pulling (use local image only)
trivy image --no-progress myapp:latest

# Common issues:
# "No such image" — build or pull the image first
# "TOOMANYREQUESTS" — Docker Hub rate limit; use: docker login
# Slow scan — pre-download DB with: trivy image --download-db-only
# False positives — add CVE to .trivyignore file
```

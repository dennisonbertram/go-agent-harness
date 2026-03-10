---
name: docker-push
description: "Push Docker images to registries: DockerHub, ghcr.io (GitHub Container Registry), AWS ECR. Authentication, tagging conventions, multi-platform builds. Trigger: when pushing Docker images, publishing container images, docker push, ghcr.io, ECR, DockerHub registry"
version: 1
argument-hint: "[registry/image:tag]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Docker Push

You are now operating in Docker registry push mode.

## DockerHub

```bash
# Login to DockerHub
docker login

# Login with credentials (for CI)
echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin

# Tag for DockerHub (username/repo:tag)
docker tag myapp:latest dennisonbertram/myapp:latest
docker tag myapp:latest dennisonbertram/myapp:1.2.3

# Push all tags
docker push dennisonbertram/myapp:latest
docker push dennisonbertram/myapp:1.2.3
```

## GitHub Container Registry (ghcr.io)

```bash
# Login with GitHub Personal Access Token
echo "$GITHUB_TOKEN" | docker login ghcr.io -u "$GITHUB_ACTOR" --password-stdin

# Tag for ghcr.io
docker tag myapp:latest ghcr.io/myorg/myapp:latest
docker tag myapp:latest ghcr.io/myorg/myapp:1.2.3

# Push to ghcr.io
docker push ghcr.io/myorg/myapp:latest
docker push ghcr.io/myorg/myapp:1.2.3
```

## AWS ECR (Elastic Container Registry)

```bash
# Login to ECR (uses AWS CLI credentials)
AWS_ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
AWS_REGION=${AWS_DEFAULT_REGION:-us-east-1}
aws ecr get-login-password --region $AWS_REGION \
  | docker login --username AWS --password-stdin \
    "$AWS_ACCOUNT.dkr.ecr.$AWS_REGION.amazonaws.com"

# Create ECR repository (if it doesn't exist)
aws ecr create-repository --repository-name myapp --region $AWS_REGION 2>/dev/null || true

# Tag for ECR
ECR_URI="$AWS_ACCOUNT.dkr.ecr.$AWS_REGION.amazonaws.com/myapp"
docker tag myapp:latest "$ECR_URI:latest"
docker tag myapp:latest "$ECR_URI:1.2.3"

# Push to ECR
docker push "$ECR_URI:latest"
docker push "$ECR_URI:1.2.3"
```

## Tagging Conventions

```bash
# Immutable: Git SHA tag (never overwrite)
GIT_SHA=$(git rev-parse --short HEAD)
docker tag myapp:latest myregistry/myapp:${GIT_SHA}
docker push myregistry/myapp:${GIT_SHA}

# Mutable: branch/environment tags
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD | tr '/' '-')
docker tag myapp:latest myregistry/myapp:${GIT_BRANCH}
docker push myregistry/myapp:${GIT_BRANCH}

# Semantic version (from git tag)
VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "dev")
docker tag myapp:latest myregistry/myapp:${VERSION}
docker tag myapp:latest myregistry/myapp:latest
docker push myregistry/myapp:${VERSION}
docker push myregistry/myapp:latest
```

## Build and Push in One Step

```bash
# Build and push with BuildKit
DOCKER_BUILDKIT=1 docker build \
  --push \
  -t ghcr.io/myorg/myapp:latest \
  -t ghcr.io/myorg/myapp:${GIT_SHA} \
  .
```

## Multi-Platform Builds and Push

```bash
# Create a multi-platform builder
docker buildx create --name multiplatform --use

# Build and push for linux/amd64 and linux/arm64
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --push \
  -t ghcr.io/myorg/myapp:latest \
  .

# Inspect a multi-platform manifest
docker manifest inspect ghcr.io/myorg/myapp:latest
```

## GitHub Actions Workflow Pattern

```yaml
# .github/workflows/docker-push.yml
name: Build and Push

on:
  push:
    branches: [main]
    tags: ['v*']

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4

      - name: Login to ghcr.io
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: |
            ghcr.io/${{ github.repository }}:latest
            ghcr.io/${{ github.repository }}:${{ github.sha }}
```

## Verify Push

```bash
# Verify the image exists in the registry
docker manifest inspect ghcr.io/myorg/myapp:latest

# Pull to verify (in a clean environment)
docker pull ghcr.io/myorg/myapp:latest
docker run --rm ghcr.io/myorg/myapp:latest --version

# List tags in a registry (ghcr.io via GitHub API)
gh api /user/packages/container/myapp/versions --jq '.[].metadata.container.tags[]'
```

## Security Best Practices

- Never embed credentials in Dockerfiles or image layers.
- Use short-lived tokens (`GITHUB_TOKEN` in Actions, IAM roles in ECR) rather than long-lived passwords.
- Sign images with `cosign` for supply chain security.
- Scan images for vulnerabilities before pushing to production registries.
- Use immutable tags (git SHA) for deployments; never rely solely on `latest`.

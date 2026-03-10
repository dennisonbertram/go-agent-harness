---
name: docker-build
description: "Build Docker images with docker build: multi-stage builds, build args, caching, tagging strategies. Trigger: when building Docker images, creating Dockerfiles, multi-stage builds, docker build args, build cache"
version: 1
argument-hint: "[image-name:tag or Dockerfile path]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Docker Build

You are now operating in Docker image build mode.

## Basic Build

```bash
# Build from current directory (uses ./Dockerfile)
docker build -t myapp:latest .

# Build from a specific Dockerfile
docker build -f docker/Dockerfile.prod -t myapp:prod .

# Build with a specific context directory
docker build -t myapp:latest ./src
```

## Tagging Conventions

```bash
# Semantic version tag + latest
docker build -t myapp:1.2.3 -t myapp:latest .

# Git commit hash tag (immutable)
GIT_SHA=$(git rev-parse --short HEAD)
docker build -t myapp:${GIT_SHA} -t myapp:latest .

# Branch-based tag
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD | tr '/' '-')
docker build -t myapp:${GIT_BRANCH} .
```

## Build Arguments

```bash
# Pass build-time variables
docker build \
  --build-arg APP_VERSION=1.2.3 \
  --build-arg NODE_ENV=production \
  -t myapp:latest .

# In Dockerfile, declare ARGs before using them
# ARG APP_VERSION
# ARG NODE_ENV=development
# ENV NODE_ENV=${NODE_ENV}
```

## Multi-Stage Build Dockerfile Pattern

```dockerfile
# Stage 1: Builder
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Stage 2: Final image (minimal)
FROM alpine:3.19 AS final
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /server .
EXPOSE 8080
USER nobody
ENTRYPOINT ["/app/server"]
```

```bash
# Build only the final stage (default)
docker build -t myapp:latest .

# Build a specific intermediate stage (for debugging)
docker build --target builder -t myapp:builder .
```

## Build Caching

```bash
# Use a registry cache source (BuildKit)
docker build \
  --cache-from myapp:latest \
  -t myapp:latest .

# Enable BuildKit for faster builds
DOCKER_BUILDKIT=1 docker build -t myapp:latest .

# Disable cache (force rebuild all layers)
docker build --no-cache -t myapp:latest .

# Use inline cache (exports cache metadata in image)
DOCKER_BUILDKIT=1 docker build \
  --build-arg BUILDKIT_INLINE_CACHE=1 \
  -t myapp:latest .
```

## Build Output and Progress

```bash
# Quiet mode (only print final image ID)
docker build -q -t myapp:latest .

# Verbose progress with timestamps
docker build --progress=plain -t myapp:latest .

# Output build context size
docker build -t myapp:latest . 2>&1 | grep "Sending build context"
```

## Inspect Build Results

```bash
# Show image layers and sizes
docker history myapp:latest

# Show image metadata
docker inspect myapp:latest

# Check final image size
docker images myapp:latest

# Show image labels
docker inspect --format='{{json .Config.Labels}}' myapp:latest | jq .
```

## .dockerignore

Create a `.dockerignore` file to exclude unnecessary files from the build context:

```
.git
.gitignore
*.md
node_modules
dist
coverage
*.test
.env
.env.*
```

This reduces build context size and prevents accidental secret inclusion.

## Common Build Issues

```bash
# Check if the build context is too large
du -sh . --exclude=.git

# Verify Dockerfile syntax
docker build --dry-run -t myapp:latest . 2>&1 || true

# Debug a failing layer by building to that stage
docker build --target failing-stage -t debug:latest .
docker run --rm -it debug:latest sh
```

## Best Practices

- Pin base image versions with digests in production: `FROM golang:1.22-alpine@sha256:...`
- Use non-root users in the final image: `USER nobody`
- Set `WORKDIR` explicitly instead of relying on root directory.
- Keep images small: use `alpine` or `distroless` base images.
- Order Dockerfile layers from least to most frequently changing (dependencies before source code).
- Use `CGO_ENABLED=0` for fully static Go binaries.

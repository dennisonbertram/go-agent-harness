---
name: scaffold-go
description: "Initialize a new Go project with standard directory layout, Makefile, GitHub Actions CI, test setup, and linter configuration. Trigger: create a new Go project, init a Go module, scaffold a Go service, set up a Go project, new Go service, initialize Go project"
version: 1
argument-hint: "<module-path> [app-name]"
allowed-tools:
  - bash
  - write
  - read
---

# Go Project Scaffolding

Scaffold a complete, production-ready Go project from scratch.

**Usage**: `/scaffold-go <module-path> [app-name]`

- `module-path`: The Go module path, e.g. `github.com/acme/myapp` or `myapp`
- `app-name`: Optional. Defaults to the last segment of the module path.

## Steps

### 1. Determine Names

Parse the arguments:
- `MODULE_PATH` = first argument (e.g. `github.com/acme/myapp`)
- `APP_NAME` = second argument if provided, otherwise the last path segment of MODULE_PATH (e.g. `myapp`)
- `DIR` = APP_NAME (the directory to create)

### 2. Confirm Before Creating

Tell the user:
- Module path: `<MODULE_PATH>`
- App name: `<APP_NAME>`
- Directory: `./<DIR>/`

Ask: "Proceed with scaffolding? (yes/no)"

Wait for confirmation before continuing.

### 3. Create Directory Structure

```bash
mkdir -p <DIR>/cmd/<APP_NAME>
mkdir -p <DIR>/internal/app
mkdir -p <DIR>/.github/workflows
```

Only create `pkg/` if the user explicitly asked for a library. Skip it for services/CLIs.

### 4. Initialize Go Module

```bash
cd <DIR> && go mod init <MODULE_PATH>
```

### 5. Create Source Files

**`cmd/<APP_NAME>/main.go`**:
```go
package main

import (
	"fmt"
	"os"

	"<MODULE_PATH>/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

**`internal/app/app.go`**:
```go
package app

import "fmt"

// Run is the application entry point.
func Run() error {
	fmt.Println("Hello from <APP_NAME>!")
	return nil
}
```

**`internal/app/app_test.go`**:
```go
package app_test

import "testing"

func TestRun(t *testing.T) {
	if err := Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
```

Fix the import in `app_test.go` — it must import `"<MODULE_PATH>/internal/app"` and call `app.Run()`:
```go
package app_test

import (
	"testing"

	"<MODULE_PATH>/internal/app"
)

func TestRun(t *testing.T) {
	if err := app.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
```

### 6. Create Makefile

**`Makefile`**:
```makefile
.PHONY: build test lint clean coverage docker

APP_NAME := <APP_NAME>
MODULE   := <MODULE_PATH>

build:
	go build -o bin/$(APP_NAME) ./cmd/$(APP_NAME)

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

docker:
	docker build -t $(APP_NAME):latest .
```

### 7. Create GitHub Actions CI

**`.github/workflows/ci.yml`**:
```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Run tests
        run: go test ./... -race -count=1

      - name: Run linter
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

### 8. Create Golangci-lint Config

**`.golangci.yml`**:
```yaml
version: "2"

linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck

linters-settings:
  govet:
    enable-all: true

run:
  timeout: 5m
```

### 9. Create .gitignore

**`.gitignore`**:
```
# Binaries
bin/
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test artifacts
coverage.out
coverage.html

# Go workspace
vendor/

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
```

### 10. Create Dockerfile

**`Dockerfile`** (multi-stage, distroless final image):
```dockerfile
# Stage 1: Build
FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/<APP_NAME> ./cmd/<APP_NAME>

# Stage 2: Run
FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/<APP_NAME> /<APP_NAME>

ENTRYPOINT ["/<APP_NAME>"]
```

### 11. Create CLAUDE.md

**`CLAUDE.md`** (project instructions for AI agents):
```markdown
# CLAUDE.md

## Project

<APP_NAME> — Go service/application.

## Module

`<MODULE_PATH>`

## Commands

- `go test ./... -race` — run tests with race detector
- `make build` — build binary to `bin/<APP_NAME>`
- `make test` — run tests
- `make lint` — run golangci-lint
- `make coverage` — generate coverage report

## Structure

- `cmd/<APP_NAME>/` — main entry point
- `internal/app/` — core application logic
- `.github/workflows/` — CI configuration

## Engineering Rules

- TDD: write tests before implementation
- Race detector must pass: `go test ./... -race`
- No broken tests — fix before adding new features
```

### 12. Create README.md

**`README.md`**:
```markdown
# <APP_NAME>

A Go application.

## Prerequisites

- Go 1.23+
- [golangci-lint](https://golangci-lint.run/usage/install/) (for linting)
- Docker (optional, for container builds)

## Getting Started

```bash
# Build
make build

# Run
./bin/<APP_NAME>

# Test
make test

# Lint
make lint
```

## Project Structure

```
<APP_NAME>/
├── cmd/<APP_NAME>/     # Entry point
├── internal/app/       # Application logic
├── .github/workflows/  # CI pipeline
├── Makefile
├── Dockerfile
└── README.md
```
```

### 13. Tidy Dependencies

```bash
cd <DIR> && go mod tidy
```

### 14. Initialize Git Repository

```bash
cd <DIR> && git init && git add . && git commit -m "Initial commit: scaffold Go project"
```

### 15. Verify

Run the tests to confirm everything works:
```bash
cd <DIR> && go test ./... -race
```

Expected output: `ok  <MODULE_PATH>/internal/app`

Report to the user:
- Directory created: `./<DIR>/`
- Module: `<MODULE_PATH>`
- Tests: passing
- Next steps: implement your logic in `internal/app/app.go`, add more packages under `internal/`, run `make build` to build

---
name: doc-generation
description: "Generate and review Go documentation with godoc, OpenAPI specs with swag/swaggo, and README generation patterns. Trigger: when generating Go documentation, godoc, OpenAPI docs, swagger, swaggo, README generation, API documentation"
version: 1
argument-hint: "[package or ./...]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Documentation Generation

You are now operating in documentation generation mode.

## Go Documentation (godoc)

### Viewing Package Documentation

```bash
# View package documentation in terminal
go doc ./internal/harness

# View a specific type
go doc ./internal/harness Runner

# View a specific method
go doc ./internal/harness Runner.Run

# View all exported symbols
go doc -all ./internal/harness

# View unexported symbols too
go doc -u ./internal/harness

# View documentation for a standard library package
go doc net/http
go doc net/http.Server
```

### Serving Documentation Locally

```bash
# Install pkgsite (modern godoc server)
go install golang.org/x/pkgsite/cmd/pkgsite@latest

# Serve docs for the current module
pkgsite -open .

# Or use the classic godoc server
go install golang.org/x/tools/cmd/godoc@latest
godoc -http=:6060
# Open http://localhost:6060/pkg/go-agent-harness/
```

### Writing Good godoc Comments

```go
// Package harness implements the agent execution loop.
// It coordinates LLM calls, tool dispatch, and step management.
package harness

// Runner executes an agent conversation loop.
// It drives the LLM → tool call → execute → repeat cycle
// until the step limit is reached or the agent signals completion.
type Runner struct {
    // unexported fields
}

// Run executes the agent loop for the given conversation.
// It returns an error if the context is cancelled, the step limit
// is exceeded, or a tool call fails fatally.
//
// Run is safe for concurrent use.
func (r *Runner) Run(ctx context.Context, conv *Conversation) error {
    // implementation
}
```

### Documentation Linting

```bash
# Check for doc format issues
go vet ./...

# Install and run staticcheck (catches missing docs on exported symbols)
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...

# Install golangci-lint with godot linter (doc comments end with period)
golangci-lint run --enable godot ./...
```

## OpenAPI / Swagger Documentation (swaggo)

### Installation

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

### Annotate Handlers

```go
// @Summary     List users
// @Description Get a paginated list of all users
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       page  query  int  false  "Page number (default: 1)"
// @Param       limit query  int  false  "Page size (default: 20)"
// @Success     200   {object}  ListUsersResponse
// @Failure     401   {object}  ErrorResponse
// @Failure     500   {object}  ErrorResponse
// @Router      /api/v1/users [get]
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
    // implementation
}
```

### Generate OpenAPI Spec

```bash
# Generate docs/ directory with swagger.json and swagger.yaml
swag init -g cmd/harnessd/main.go

# Specify output directory
swag init -g cmd/harnessd/main.go -o api/docs

# Format swag annotations
swag fmt
```

### Serve Swagger UI

```go
import (
    swaggerFiles "github.com/swaggo/files"
    ginSwagger "github.com/swaggo/gin-swagger"
    _ "go-agent-harness/api/docs"  // generated docs
)

// Register Swagger UI route
r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
// Open http://localhost:8080/swagger/index.html
```

## README Generation Patterns

### Package README Skeleton

```markdown
# package-name

Brief one-sentence description.

## Overview

What problem this package solves and when to use it.

## Installation

```bash
go get go-agent-harness/internal/package-name
```

## Usage

```go
// Minimal working example
```

## API Reference

See [godoc](https://pkg.go.dev/go-agent-harness/internal/package-name) for full API docs.

## Contributing

Run tests: `go test ./...`
```

### Auto-generate Changelog Entry

```bash
# Generate changelog from git log since last tag
LAST_TAG=$(git describe --tags --abbrev=0)
git log ${LAST_TAG}..HEAD --pretty=format:"- %s (%h)" --no-merges
```

## Documentation Checklist

Before merging a new package or significant API change:

```bash
# 1. All exported symbols have doc comments
grep -n '^func \|^type \|^var \|^const ' *.go | grep -v '//'

# 2. Run go vet to catch basic issues
go vet ./...

# 3. View rendered docs
go doc -all ./...

# 4. Check for TODO/FIXME left in exported symbols
grep -rn 'TODO\|FIXME' --include='*.go' .

# 5. Verify examples compile
go test -run=Example ./...
```

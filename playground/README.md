# Playground

`playground/` holds experimental, training, and snippet-style Go code that is useful for practice and exploration but is not part of the main harness product surface.

Why it is isolated:

- The main application lives under `cmd/`, `internal/`, `plugins/`, and supporting repo directories.
- The playground code has intentionally looser quality and packaging constraints than product code.
- Keeping it in a separate Go module prevents example breakage from polluting product-level verification and keeps the repo root focused on the real application layout.

Working with it:

```bash
cd playground
go test ./...
```

Treat the playground as a sandbox. Product-quality changes should land in the main module instead.

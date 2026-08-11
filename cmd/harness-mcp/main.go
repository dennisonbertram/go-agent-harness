// Command harness-mcp is a stdio MCP server that proxies to harnessd.
// It speaks JSON-RPC 2.0 over stdin/stdout and exposes the harnessd REST API
// as MCP tools, making it compatible with Claude Desktop and other MCP clients.
//
// Configuration via environment variables:
//
//	HARNESS_ADDR     - harnessd base URL (default: http://localhost:8080)
//	HARNESS_API_KEY  - bearer token for a harnessd that requires auth (optional)
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	harnessmcp "go-agent-harness/internal/harnessmcp"
)

var (
	runMain                = run
	exitFunc               = os.Exit
	stdinReader  io.Reader = os.Stdin
	stdoutWriter io.Writer = os.Stdout
	getenvFunc             = os.Getenv
)

func main() {
	if err := runMain(); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "harness-mcp: %v\n", err)
		exitFunc(1)
	}
}

func run() error {
	addr := getenvFunc("HARNESS_ADDR")
	if addr == "" {
		addr = "http://localhost:8080"
	}

	// harnessd enforces Bearer auth unless explicitly disabled, so without a
	// token this reaches only an auth-disabled daemon (issue #1319). Read from
	// the environment, never from a tool argument: a model must not be able to
	// set or read a credential.
	token := getenvFunc("HARNESS_API_KEY")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	return runWithIO(ctx, stdinReader, stdoutWriter, addr, token)
}

// runWithIO runs the MCP server reading from in and writing to out. Extracted
// for testability.
func runWithIO(ctx context.Context, in io.Reader, out io.Writer, addr, token string) error {
	client := harnessmcp.NewHarnessClientWithToken(addr, token)
	dispatcher := harnessmcp.NewDispatcher(client, harnessmcp.RealClock{})
	transport := harnessmcp.NewStdioTransport(in, out, dispatcher)
	return transport.Run(ctx)
}

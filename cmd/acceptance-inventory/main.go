// acceptance-inventory compiles a report from a running harnessd tool catalog
// and the built-in harnesscli command registry. It performs no tool execution.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/internal/acceptance/inventory"
)

var (
	commandArgs           = os.Args[1:]
	runMain               = run
	stdout      io.Writer = os.Stdout
	stderr      io.Writer = os.Stderr
	exitFunc              = os.Exit
)

func main() {
	flags := flag.NewFlagSet("acceptance-inventory", flag.ContinueOnError)
	flags.SetOutput(stderr)
	endpoint := flags.String("harness-url", "http://127.0.0.1:8080", "running harnessd base URL")
	if err := flags.Parse(commandArgs); err != nil {
		exitFunc(2)
		return
	}
	if err := runMain(stdout, *endpoint); err != nil {
		fmt.Fprintln(stderr, "acceptance-inventory:", err)
		exitFunc(1)
	}
}

func run(out io.Writer, endpoint string) error {
	base := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if base == "" {
		return fmt.Errorf("harness URL is required")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(base + "/v1/tools")
	if err != nil {
		return fmt.Errorf("GET /v1/tools: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("GET /v1/tools: status %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Tools                         []inventory.HTTPTool             `json:"tools"`
		ConfiguredUnavailableToolsets *[]inventory.ConfiguredToolset   `json:"configured_unavailable_toolsets"`
		Unavailable                   *[]inventory.ResolverObservation `json:"unavailable"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return fmt.Errorf("decode /v1/tools: %w", err)
	}
	if payload.ConfiguredUnavailableToolsets == nil || payload.Unavailable == nil {
		return fmt.Errorf("decode /v1/tools: authoritative resolver evidence is absent or null")
	}
	compiled, err := inventory.Compile(inventory.InputFromHTTPBoundary(
		payload.Tools,
		tui.NewCommandRegistry().All(),
		*payload.Unavailable,
		*payload.ConfiguredUnavailableToolsets,
	))
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, inventory.RenderMarkdown(compiled))
	return err
}

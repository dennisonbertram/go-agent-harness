// acceptance-api-sse reports exact live API-case coverage. Execution remains
// fixture-owned; this command refuses to call an incomplete manifest green.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"go-agent-harness/internal/acceptance/apisserunner"
)

var (
	commandArgs           = os.Args[1:]
	runMain               = run
	stdout      io.Writer = os.Stdout
	stderr      io.Writer = os.Stderr
	exitFunc              = os.Exit
)

func main() {
	flags := flag.NewFlagSet("acceptance-api-sse", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("harness-url", "http://127.0.0.1:8080", "running harnessd base URL")
	manifestPath := flags.String("manifest", "", "reviewed API case manifest JSON")
	if err := flags.Parse(commandArgs); err != nil {
		exitFunc(2)
		return
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "acceptance-api-sse: -manifest is required")
		exitFunc(2)
		return
	}
	report, err := runMain(context.Background(), *base, *manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "acceptance-api-sse:", err)
		exitFunc(1)
		return
	}
	if err := json.NewEncoder(stdout).Encode(report); err != nil {
		fmt.Fprintln(stderr, "acceptance-api-sse:", err)
		exitFunc(1)
	}
	if len(report.Missing) != 0 {
		os.Exit(1)
	}
}

func run(ctx context.Context, base, manifestPath string) (apisserunner.CoverageReport, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return apisserunner.CoverageReport{}, err
	}
	defer file.Close()
	var manifest apisserunner.Manifest
	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return apisserunner.CoverageReport{}, fmt.Errorf("decode manifest: %w", err)
	}
	compiled, err := (apisserunner.Runner{BaseURL: base}).LoadLiveInventory(ctx)
	if err != nil {
		return apisserunner.CoverageReport{}, err
	}
	return apisserunner.BuildCoverageReport(compiled, manifest)
}

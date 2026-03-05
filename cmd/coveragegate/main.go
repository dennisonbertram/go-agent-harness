package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"go-agent-harness/internal/quality/coveragegate"
)

var (
	coverageReportFn           = coverageFuncReport
	runCommand                 = run
	exitFunc                   = os.Exit
	osArgs                     = os.Args
	stdout           io.Writer = os.Stdout
	stderr           io.Writer = os.Stderr
)

func main() {
	exitFunc(runCommand(osArgs[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("coveragegate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	coverprofile := flags.String("coverprofile", "coverage.out", "path to coverage profile")
	minTotal := flags.Float64("min-total", 80.0, "minimum total statement coverage percentage")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(stderr, "coveragegate: parse failed: %v\n", err)
		return 1
	}

	report, err := coverageReportFn(*coverprofile)
	if err != nil {
		fmt.Fprintf(stderr, "coveragegate: %v\n", err)
		return 1
	}

	if err := coveragegate.ValidateReport(report, *minTotal); err != nil {
		fmt.Fprintf(stderr, "coveragegate: validation failed: %v\n", err)
		fmt.Fprintln(stderr, report)
		return 1
	}

	parsed, err := coveragegate.ParseReport(report)
	if err != nil {
		fmt.Fprintf(stderr, "coveragegate: parse failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "coveragegate: PASS (total=%.1f%%, min=%.1f%%, zero-functions=%d)\n", parsed.TotalCoverage, *minTotal, len(parsed.ZeroFunctions))
	return 0
}

func coverageFuncReport(profilePath string) (string, error) {
	cmd := exec.Command("go", "tool", "cover", "-func="+profilePath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("go tool cover failed: %s", stderr.String())
		}
		return "", fmt.Errorf("go tool cover failed: %w", err)
	}
	return out.String(), nil
}

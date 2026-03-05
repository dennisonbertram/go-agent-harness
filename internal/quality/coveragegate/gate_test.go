package coveragegate

import (
	"strings"
	"testing"
)

const sampleCoverage = `go-agent-harness/cmd/harnessd/main.go:28:            main                    100.0%
go-agent-harness/cmd/harnessd/main.go:35:            run                     80.0%
go-agent-harness/internal/server/http.go:26:          handleHealth            100.0%
go-agent-harness/internal/server/http.go:170:         writeError              0.0%
total:                                                  (statements)            81.0%`

func TestParseReport(t *testing.T) {
	t.Parallel()

	result, err := ParseReport(sampleCoverage)
	if err != nil {
		t.Fatalf("parse report: %v", err)
	}
	if result.TotalCoverage != 81.0 {
		t.Fatalf("expected total 81.0, got %.1f", result.TotalCoverage)
	}
	if len(result.ZeroFunctions) != 1 {
		t.Fatalf("expected 1 zero function, got %d", len(result.ZeroFunctions))
	}
	if !strings.Contains(result.ZeroFunctions[0], "writeError") {
		t.Fatalf("unexpected zero function entry: %q", result.ZeroFunctions[0])
	}
}

func TestValidateReport(t *testing.T) {
	t.Parallel()

	if err := ValidateReport(strings.ReplaceAll(sampleCoverage, "0.0%", "20.0%"), 80.0); err != nil {
		t.Fatalf("expected report to pass: %v", err)
	}
	if err := ValidateReport(sampleCoverage, 80.0); err == nil {
		t.Fatalf("expected zero-function failure")
	}
	if err := ValidateReport(strings.ReplaceAll(sampleCoverage, "81.0%", "79.9%"), 80.0); err == nil {
		t.Fatalf("expected total coverage threshold failure")
	}
}

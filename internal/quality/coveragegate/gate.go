package coveragegate

import (
	"fmt"
	"strconv"
	"strings"
)

type ReportResult struct {
	TotalCoverage float64
	ZeroFunctions []string
}

func ParseReport(report string) (ReportResult, error) {
	lines := strings.Split(report, "\n")
	result := ReportResult{ZeroFunctions: make([]string, 0)}
	foundTotal := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		last := fields[len(fields)-1]
		coverage, err := parsePercent(last)
		if err != nil {
			continue
		}

		if strings.HasPrefix(line, "total:") {
			result.TotalCoverage = coverage
			foundTotal = true
			continue
		}

		if strings.Contains(fields[0], ".go:") && coverage <= 0.0 {
			result.ZeroFunctions = append(result.ZeroFunctions, line)
		}
	}

	if !foundTotal {
		return ReportResult{}, fmt.Errorf("coverage report missing total line")
	}

	return result, nil
}

func ValidateReport(report string, minTotal float64) error {
	parsed, err := ParseReport(report)
	if err != nil {
		return err
	}

	if parsed.TotalCoverage < minTotal {
		return fmt.Errorf("total coverage %.1f%% is below threshold %.1f%%", parsed.TotalCoverage, minTotal)
	}
	if len(parsed.ZeroFunctions) > 0 {
		return fmt.Errorf("functions with zero coverage detected:\n%s", strings.Join(parsed.ZeroFunctions, "\n"))
	}
	return nil
}

func parsePercent(token string) (float64, error) {
	token = strings.TrimSpace(strings.TrimSuffix(token, "%"))
	if token == "" {
		return 0, fmt.Errorf("empty percent token")
	}
	value, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid percent %q: %w", token, err)
	}
	return value, nil
}

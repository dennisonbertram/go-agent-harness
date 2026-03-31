package behavioral_specs

import (
	"embed"
	"strings"
)

//go:embed *.md
var specsFS embed.FS

// LoadSpec loads and parses the behavioral spec for the named tool.
// Returns (nil, nil) when no spec file exists for the tool — graceful degradation.
// Returns a non-nil error only for parse failures on existing files.
func LoadSpec(toolName string) (*BehavioralSpec, error) {
	data, err := specsFS.ReadFile(toolName + ".md")
	if err != nil {
		// File not found — graceful degradation, not an error.
		return nil, nil
	}
	return ParseMarkdown(string(data))
}

// ParseMarkdown parses a behavioral spec markdown string into a BehavioralSpec.
// Sections are identified by "## Heading" markers. Unknown sections are ignored.
func ParseMarkdown(markdown string) (*BehavioralSpec, error) {
	spec := &BehavioralSpec{}

	lines := strings.Split(markdown, "\n")

	type section struct {
		name  string
		lines []string
	}

	var sections []section
	var current *section

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			sections = append(sections, section{name: strings.TrimSpace(heading)})
			current = &sections[len(sections)-1]
			continue
		}
		if current != nil {
			current.lines = append(current.lines, line)
		}
	}

	for i := range sections {
		s := &sections[i]
		body := strings.TrimSpace(strings.Join(s.lines, "\n"))

		switch s.name {
		case "When to Use":
			spec.WhenToUse = body
		case "When NOT to Use":
			spec.WhenNotToUse = body
		case "Behavioral Rules":
			spec.BehavioralRules = parseListItems(body)
		case "Common Mistakes":
			spec.CommonMistakes = parseListItems(body)
		case "Examples":
			spec.Examples = parseExamples(body)
		}
	}

	return spec, nil
}

// parseListItems extracts list items from a markdown section body.
// Handles both "- item" and "1. item" formats. Returns each item's text.
func parseListItems(body string) []string {
	var items []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Handle "- item" format
		if strings.HasPrefix(line, "- ") {
			items = append(items, strings.TrimPrefix(line, "- "))
			continue
		}
		// Handle "1. item", "2. item", etc.
		if len(line) > 2 && line[1] == '.' && line[0] >= '0' && line[0] <= '9' {
			items = append(items, strings.TrimSpace(line[2:]))
			continue
		}
		// Handle "10. item" etc (two-digit numbers)
		if len(line) > 3 && line[2] == '.' && line[0] >= '0' && line[0] <= '9' && line[1] >= '0' && line[1] <= '9' {
			items = append(items, strings.TrimSpace(line[3:]))
			continue
		}
	}
	return items
}

// parseExamples parses ### WRONG / ### RIGHT paired blocks from a section body.
// Multiple example pairs can appear in a single Examples section.
func parseExamples(body string) []SpecExample {
	var examples []SpecExample
	var current *SpecExample
	var inWrong, inRight bool
	var wrongLines, rightLines []string

	flushCurrent := func() {
		if current != nil {
			current.Wrong = strings.TrimSpace(strings.Join(wrongLines, "\n"))
			current.Right = strings.TrimSpace(strings.Join(rightLines, "\n"))
			examples = append(examples, *current)
			current = nil
			wrongLines = nil
			rightLines = nil
			inWrong = false
			inRight = false
		}
	}

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "### WRONG") {
			if current != nil && inRight {
				flushCurrent()
			}
			if current == nil {
				current = &SpecExample{Label: ""}
			}
			inWrong = true
			inRight = false
			continue
		}
		if strings.HasPrefix(trimmed, "### RIGHT") {
			inWrong = false
			inRight = true
			continue
		}

		if inWrong {
			wrongLines = append(wrongLines, line)
		} else if inRight {
			rightLines = append(rightLines, line)
		}
	}

	flushCurrent()
	return examples
}

// FormatSpec renders a BehavioralSpec to a string according to the config toggles.
// Returns "" when cfg.Enabled is false or spec is nil.
func FormatSpec(spec *BehavioralSpec, cfg BehavioralSpecConfig) string {
	if !cfg.Enabled || spec == nil {
		return ""
	}

	var sb strings.Builder

	if spec.WhenToUse != "" {
		sb.WriteString("## When to Use\n")
		sb.WriteString(spec.WhenToUse)
		sb.WriteString("\n\n")
	}

	if cfg.IncludeWhenNotToUse && spec.WhenNotToUse != "" {
		sb.WriteString("## When NOT to Use\n")
		sb.WriteString(spec.WhenNotToUse)
		sb.WriteString("\n\n")
	}

	if len(spec.BehavioralRules) > 0 {
		sb.WriteString("## Behavioral Rules\n")
		for i, rule := range spec.BehavioralRules {
			sb.WriteString(strings.Join([]string{numStr(i + 1), ". ", rule, "\n"}, ""))
		}
		sb.WriteString("\n")
	}

	if (cfg.IncludeAntiPatterns || cfg.IncludeCommonMistakes) && len(spec.CommonMistakes) > 0 {
		sb.WriteString("## Common Mistakes\n")
		for _, m := range spec.CommonMistakes {
			sb.WriteString("- ")
			sb.WriteString(m)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(spec.Examples) > 0 {
		sb.WriteString("## Examples\n")
		for _, ex := range spec.Examples {
			if ex.Label != "" {
				sb.WriteString("### ")
				sb.WriteString(ex.Label)
				sb.WriteString("\n")
			}
			if ex.Wrong != "" {
				sb.WriteString("### WRONG\n")
				sb.WriteString(ex.Wrong)
				sb.WriteString("\n\n")
			}
			if ex.Right != "" {
				sb.WriteString("### RIGHT\n")
				sb.WriteString(ex.Right)
				sb.WriteString("\n\n")
			}
		}
	}

	result := strings.TrimSpace(sb.String())
	if cfg.MaxSpecLength > 0 && len(result) > cfg.MaxSpecLength {
		return result[:cfg.MaxSpecLength]
	}
	return result
}

// numStr converts an int to its string representation without importing strconv.
func numStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

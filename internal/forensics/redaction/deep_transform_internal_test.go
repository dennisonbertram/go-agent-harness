package redaction

import (
	"strings"
	"testing"
)

func TestDeepTransformValueTransformsTypedContainers(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"labels": []string{"secret", "plain"},
		"env":    map[string]string{"token": "secret"},
	}

	got := deepTransformValue(input, strings.ToUpper)
	out, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", got)
	}

	labels, ok := out["labels"].([]string)
	if !ok {
		t.Fatalf("expected []string labels, got %T", out["labels"])
	}
	if labels[0] != "SECRET" || labels[1] != "PLAIN" {
		t.Fatalf("unexpected transformed labels: %v", labels)
	}

	env, ok := out["env"].(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string env, got %T", out["env"])
	}
	if env["token"] != "SECRET" {
		t.Fatalf("expected transformed map value, got %q", env["token"])
	}
}

package streamrenderer_test

import (
	"reflect"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/streamrenderer"
)

func TestTUI014_LinesReturnsSplitContent(t *testing.T) {
	t.Parallel()

	sr := streamrenderer.New()
	if got := sr.Lines(); got != nil {
		t.Fatalf("expected nil lines for empty renderer, got %v", got)
	}

	sr.AppendDelta("alpha\nbeta")
	if got := sr.Lines(); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("unexpected split lines: %v", got)
	}
}

package layout_test

import (
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/layout"
)

func TestLayoutCoverageHelpers(t *testing.T) {
	t.Parallel()

	c := layout.DefaultConstraints()
	if c.MinWidth != 80 || c.MinHeight != 24 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if got := c.ViewportHeight(0); got != 1 {
		t.Fatalf("expected viewport height clamp to 1, got %d", got)
	}

	container := layout.NewContainer(layout.Layout{Width: 77})
	if container.ViewportWidth() != 77 {
		t.Fatalf("unexpected viewport width: %d", container.ViewportWidth())
	}
	if container.InputWidth() != 77 {
		t.Fatalf("unexpected input width: %d", container.InputWidth())
	}
}

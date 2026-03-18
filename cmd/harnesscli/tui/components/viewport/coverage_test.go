package viewport_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/viewport"
)

func TestViewportCoverageHelpers(t *testing.T) {
	t.Parallel()

	vp := viewport.New(80, 2)
	vp = vp.SetMaxHistory(2)
	vp.AppendLine("one")
	vp.AppendLine("two")
	vp.AppendLine("three")

	rendered := vp.View()
	if strings.Contains(rendered, "one") {
		t.Fatalf("expected max history pruning to drop oldest line, got %q", rendered)
	}

	vm := viewport.NewVirtualizedModel(2, 10)
	if !vm.AtBottom() {
		t.Fatal("expected new virtualized model to start at bottom")
	}
	vm = vm.AppendLine("alpha")
	vm = vm.ScrollUp(1)
	if vm.AtBottom() {
		t.Fatal("expected scroll up to disable bottom pinning")
	}
	vm = vm.ScrollToBottom()
	if !vm.AtBottom() {
		t.Fatal("expected scroll to bottom to re-enable bottom pinning")
	}
}

package spinner

import (
	"testing"
)

// cycleGlyphs ticks a started spinner until the pulse returns to its first
// step, returning the glyph observed on every tick.
func cycleGlyphs(t *testing.T) []string {
	t.Helper()
	m := New(0).Start().SetAction("Thinking")

	var seen []string
	for i := 0; i < 200; i++ { // generous bound; a cycle is far shorter
		seen = append(seen, m.Glyph())
		m = m.Tick()
		if m.step == 0 && m.stepTicks == 0 && i > 0 {
			return seen
		}
	}
	t.Fatal("spinner never completed a cycle within 200 ticks")
	return nil
}

// TestSpinnerCycleTakesAboutTwoSeconds pins the slower cadence asked for in
// issue #1420. At the unchanged 120ms tick, a cycle should run about 2s rather
// than the previous 720ms.
func TestSpinnerCycleTakesAboutTwoSeconds(t *testing.T) {
	got := len(cycleGlyphs(t))
	const want = 18 // 18 ticks x 120ms = 2.16s
	if got != want {
		t.Fatalf("cycle is %d ticks (%.2fs at 120ms), want %d (%.2fs)",
			got, float64(got)*0.12, want, float64(want)*0.12)
	}
}

// TestSpinnerEasesAtTheExtremes pins the easing: the animation lingers at the
// top and bottom of the breath and moves quickly through the middle. A flat
// cadence — every frame held equally — is what made it read as a tick.
func TestSpinnerEasesAtTheExtremes(t *testing.T) {
	holds := map[string]int{}
	for _, g := range cycleGlyphs(t) {
		holds[g]++
	}

	lightest, heaviest := pulse[0], pulse[len(pulse)/2]
	for _, mid := range []string{"✳", "✻"} {
		if holds[lightest] <= holds[mid] {
			t.Errorf("extreme %q held %d ticks, mid-pulse %q held %d: extremes must linger longer",
				lightest, holds[lightest], mid, holds[mid])
		}
		if holds[heaviest] <= holds[mid] {
			t.Errorf("extreme %q held %d ticks, mid-pulse %q held %d: extremes must linger longer",
				heaviest, holds[heaviest], mid, holds[mid])
		}
	}
}

// TestSpinnerPulseGrowsThenShrinks pins the shape. The old order
// (✶ · ✻ ✽ ✳ ✢) jumped from the heaviest glyph straight to the lightest, which
// reads as a stutter however slowly it runs.
func TestSpinnerPulseGrowsThenShrinks(t *testing.T) {
	weight := map[string]int{"·": 0, "✢": 1, "✳": 2, "✻": 3, "✽": 4, "✶": 5}

	var seq []int
	for _, g := range pulse {
		w, ok := weight[g]
		if !ok {
			t.Fatalf("pulse contains unknown glyph %q", g)
		}
		seq = append(seq, w)
	}

	peak := 0
	for i, w := range seq {
		if w > seq[peak] {
			peak = i
		}
	}
	for i := 1; i <= peak; i++ {
		if seq[i] <= seq[i-1] {
			t.Fatalf("pulse does not grow monotonically to its peak at step %d: %v", i, seq)
		}
	}
	for i := peak + 1; i < len(seq); i++ {
		if seq[i] >= seq[i-1] {
			t.Fatalf("pulse does not shrink monotonically after its peak at step %d: %v", i, seq)
		}
	}
	// The wrap back to the start must not be a jump from heaviest to lightest.
	if seq[len(seq)-1]-seq[0] > 1 {
		t.Fatalf("wrap from %v back to %v is a jump, not a breath", seq[len(seq)-1], seq[0])
	}
}

// TestSpinnerStillAnimatesUnderEasing is the control: a hold table that never
// released would satisfy "slower" while freezing the spinner outright.
func TestSpinnerStillAnimatesUnderEasing(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range cycleGlyphs(t) {
		seen[g] = true
	}
	if len(seen) < 4 {
		t.Fatalf("only %d distinct glyphs in a full cycle; the spinner would look stalled", len(seen))
	}
}

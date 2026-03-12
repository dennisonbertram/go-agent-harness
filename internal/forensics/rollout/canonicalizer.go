package rollout

import (
	"sort"
	"time"
)

// CanonicalizationOptions controls which fields are stripped when
// canonicalizing rollout events for comparison.
type CanonicalizationOptions struct {
	StripTimestamps bool
	StripRunIDs     bool
	StripEventIDs   bool
}

// DefaultOptions strips timestamps, run IDs, and event IDs for comparison.
var DefaultOptions = CanonicalizationOptions{
	StripTimestamps: true,
	StripRunIDs:     true,
	StripEventIDs:   true,
}

// Canonicalize returns a copy of events with non-deterministic fields stripped
// according to opts, and sorted by step then original sequence order.
func Canonicalize(events []RolloutEvent, opts CanonicalizationOptions) []RolloutEvent {
	result := make([]RolloutEvent, len(events))
	for i, ev := range events {
		result[i] = canonicalizeEvent(ev, opts)
	}

	// Stable sort by step, preserving original order within a step.
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Step < result[j].Step
	})

	return result
}

func canonicalizeEvent(ev RolloutEvent, opts CanonicalizationOptions) RolloutEvent {
	out := RolloutEvent{
		ID:        ev.ID,
		Type:      ev.Type,
		Step:      ev.Step,
		Timestamp: ev.Timestamp,
	}

	if opts.StripTimestamps {
		out.Timestamp = time.Time{}
	}
	if opts.StripEventIDs {
		out.ID = ""
	}

	// Deep-copy and clean payload.
	if ev.Payload != nil {
		cleaned := make(map[string]any, len(ev.Payload))
		for k, v := range ev.Payload {
			cleaned[k] = v
		}
		if opts.StripRunIDs {
			delete(cleaned, "run_id")
		}
		if opts.StripTimestamps {
			delete(cleaned, "timestamp")
			delete(cleaned, "ts")
		}
		if opts.StripEventIDs {
			delete(cleaned, "id")
			delete(cleaned, "event_id")
		}
		out.Payload = cleaned
	}

	return out
}

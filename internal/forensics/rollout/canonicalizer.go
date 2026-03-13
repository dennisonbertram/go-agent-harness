package rollout

import (
	"sort"
	"time"
)

// deepCopyPayloadMap returns a recursive deep copy of a map[string]any,
// preventing aliasing between the canonicalized output and the original event.
func deepCopyPayloadMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyPayloadValue(v)
	}
	return out
}

func deepCopyPayloadValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyPayloadMap(val)
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = deepCopyPayloadValue(elem)
		}
		return out
	default:
		return v
	}
}

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

	// HIGH-2 fix (round 29): deep copy the payload before stripping fields.
	// The previous shallow copy shared nested map[string]any and []any values
	// with the original event. Any downstream mutation of the canonicalized
	// copy's nested structures silently corrupts the original event payload,
	// causing aliasing bugs in multi-stage replay/redaction pipelines.
	if ev.Payload != nil {
		cleaned := deepCopyPayloadMap(ev.Payload)
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

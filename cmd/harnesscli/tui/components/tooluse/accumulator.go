package tooluse

import "strings"

// callState holds the accumulated chunks and completion flag for one call ID.
type callState struct {
	chunks []string
	done   bool
}

// Accumulator accumulates streaming tool call chunks keyed by call ID.
// It is immutable — all mutations return a new copy.
type Accumulator struct {
	states map[string]callState
	order  []string // insertion-ordered call IDs
}

// NewAccumulator returns an empty Accumulator ready for use.
func NewAccumulator() Accumulator {
	return Accumulator{
		states: make(map[string]callState),
	}
}

// Append returns a new Accumulator with chunk appended to the given callID.
//
// Deduplication: if chunk is identical to the last appended chunk for callID,
// it is silently dropped (idempotency guard for redelivered chunks).
func (a Accumulator) Append(callID, chunk string) Accumulator {
	next := a.clone()

	st, exists := next.states[callID]
	if !exists {
		next.order = append(next.order, callID)
	}

	// Deduplication: skip if identical to the last chunk for this callID.
	if len(st.chunks) > 0 && st.chunks[len(st.chunks)-1] == chunk {
		return next
	}

	st.chunks = append(st.chunks, chunk)
	next.states[callID] = st
	return next
}

// AppendDone returns a new Accumulator with chunk appended and the call
// marked as complete (Done flag set to true).
func (a Accumulator) AppendDone(callID, chunk string) Accumulator {
	next := a.Append(callID, chunk)

	st := next.states[callID]
	st.done = true
	next.states[callID] = st
	return next
}

// Get returns all accumulated chunks for callID joined into a single string.
// Returns empty string if callID is unknown.
func (a Accumulator) Get(callID string) string {
	st, ok := a.states[callID]
	if !ok {
		return ""
	}
	return strings.Join(st.chunks, "")
}

// Complete returns true once the callID has been marked done via AppendDone.
// Returns false for unknown call IDs.
func (a Accumulator) Complete(callID string) bool {
	st, ok := a.states[callID]
	if !ok {
		return false
	}
	return st.done
}

// CallIDs returns all call IDs in insertion order.
func (a Accumulator) CallIDs() []string {
	result := make([]string, len(a.order))
	copy(result, a.order)
	return result
}

// Reset returns a new Accumulator with the given callID's state cleared.
// Other call IDs are preserved. The call ID is removed from the order list.
func (a Accumulator) Reset(callID string) Accumulator {
	next := a.clone()
	delete(next.states, callID)

	// Remove from order slice while preserving relative order.
	filtered := next.order[:0:len(next.order)]
	for _, id := range next.order {
		if id != callID {
			filtered = append(filtered, id)
		}
	}
	next.order = filtered
	return next
}

// clone returns a shallow copy of the Accumulator with independent maps/slices.
func (a Accumulator) clone() Accumulator {
	newStates := make(map[string]callState, len(a.states))
	for k, v := range a.states {
		// Copy the chunks slice so mutations don't affect the original.
		chunksCopy := make([]string, len(v.chunks))
		copy(chunksCopy, v.chunks)
		newStates[k] = callState{chunks: chunksCopy, done: v.done}
	}

	newOrder := make([]string, len(a.order))
	copy(newOrder, a.order)

	return Accumulator{
		states: newStates,
		order:  newOrder,
	}
}

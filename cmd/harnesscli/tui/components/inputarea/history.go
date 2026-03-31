package inputarea

// History stores a bounded list of submitted commands with navigation support.
// It is an immutable value type — all mutating methods return a new History.
type History struct {
	entries []string
	maxSize int
	pos     int    // -1 = current draft (not in history)
	draft   string // saved draft when entering history
}

// NewHistory creates a new History with the given maximum size.
// If maxSize <= 0, a default of 100 is used.
func NewHistory(maxSize int) History {
	if maxSize <= 0 {
		maxSize = 100
	}
	return History{
		entries: nil,
		maxSize: maxSize,
		pos:     -1,
	}
}

// Push adds text to the history. The current draft position is reset.
// Consecutive duplicates are skipped (same as the most recent entry).
// Returns the new History.
func (h History) Push(text string) History {
	if text == "" {
		return h
	}
	// Skip duplicate consecutive entry.
	if len(h.entries) > 0 && h.entries[0] == text {
		h.pos = -1
		h.draft = ""
		return h
	}
	// Prepend newest entry at the front.
	newEntries := make([]string, 0, len(h.entries)+1)
	newEntries = append(newEntries, text)
	newEntries = append(newEntries, h.entries...)
	// Trim to maxSize.
	if len(newEntries) > h.maxSize {
		newEntries = newEntries[:h.maxSize]
	}
	h.entries = newEntries
	h.pos = -1
	h.draft = ""
	return h
}

// Up navigates backward through history (toward older entries).
// If currently at the draft (pos == -1), saves the current text as draft and
// moves to the most recent history entry.
// Returns (new History state, text to display).
// If history is empty, returns ("", unchanged state).
func (h History) Up(currentText string) (History, string) {
	if len(h.entries) == 0 {
		return h, currentText
	}
	if h.pos == -1 {
		// Save current text as draft before entering history.
		h.draft = currentText
		h.pos = 0
	} else if h.pos < len(h.entries)-1 {
		h.pos++
	}
	return h, h.entries[h.pos]
}

// Down navigates forward through history (toward newer entries / draft).
// When navigating past the newest entry, returns the saved draft and resets pos to -1.
// Returns (new History state, text to display).
func (h History) Down() (History, string) {
	if h.pos == -1 {
		// Already at draft; nothing to do.
		return h, h.draft
	}
	if h.pos > 0 {
		h.pos--
		return h, h.entries[h.pos]
	}
	// pos == 0: we were at the newest entry; return to draft.
	h.pos = -1
	draft := h.draft
	h.draft = ""
	return h, draft
}

// AtDraft returns true when the cursor is at the current (un-navigated) position.
func (h History) AtDraft() bool {
	return h.pos == -1
}

// Len returns the number of entries in the history.
func (h History) Len() int {
	return len(h.entries)
}

// Clear removes all history entries and resets state.
func (h History) Clear() History {
	h.entries = nil
	h.pos = -1
	h.draft = ""
	return h
}

// ResetPos returns the History with the navigation position reset to draft
// (pos=-1, draft cleared) without removing existing entries.
func (h History) ResetPos() History {
	h.pos = -1
	h.draft = ""
	return h
}

// Entries returns a copy of the history entries in newest-first order.
// This is used for persistence (save to config).
func (h History) Entries() []string {
	if len(h.entries) == 0 {
		return nil
	}
	cp := make([]string, len(h.entries))
	copy(cp, h.entries)
	return cp
}

// NewHistoryWithEntries creates a History pre-populated with the given entries.
// The entries slice must be in newest-first order (as returned by Entries()).
// This is used for restoring history from persistent storage.
func NewHistoryWithEntries(maxSize int, entries []string) History {
	if maxSize <= 0 {
		maxSize = 100
	}
	h := History{
		maxSize: maxSize,
		pos:     -1,
	}
	if len(entries) > maxSize {
		entries = entries[:maxSize]
	}
	if len(entries) > 0 {
		h.entries = make([]string, len(entries))
		copy(h.entries, entries)
	}
	return h
}

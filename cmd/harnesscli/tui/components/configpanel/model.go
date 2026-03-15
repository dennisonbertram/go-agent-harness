package configpanel

import "strings"

// ConfigEntry is one editable config row.
type ConfigEntry struct {
	Key         string
	Value       string
	Description string
	ReadOnly    bool
	Dirty       bool // edited but not saved
}

// Model is the config panel state.
// All methods return a new Model (value semantics — safe for concurrent use
// when each goroutine holds its own copy).
type Model struct {
	entries      []ConfigEntry
	filtered     []ConfigEntry // current search result
	query        string
	selected     int
	editing      bool   // in inline edit mode
	editBuf      string // current edit buffer
	active       bool
	scrollOffset int
}

// New creates a new config panel Model with the given entries.
// The panel starts inactive (not shown).
func New(entries []ConfigEntry) Model {
	cp := make([]ConfigEntry, len(entries))
	copy(cp, entries)

	filt := make([]ConfigEntry, len(cp))
	copy(filt, cp)

	return Model{
		entries:  cp,
		filtered: filt,
	}
}

// Open activates the panel overlay.
func (m Model) Open() Model {
	m.active = true
	return m
}

// Close deactivates the panel overlay and exits edit mode.
func (m Model) Close() Model {
	m.active = false
	m.editing = false
	m.editBuf = ""
	return m
}

// IsActive reports whether the panel is currently visible.
func (m Model) IsActive() bool {
	return m.active
}

// SetQuery filters entries by key/value/description substring (case-insensitive).
// Resets selection to 0 and scrollOffset to 0.
func (m Model) SetQuery(q string) Model {
	m.query = q
	m.filtered = filterEntries(m.entries, q)
	m.selected = 0
	m.scrollOffset = 0
	return m
}

// SelectUp moves the selection one row up, wrapping to the last row if at top.
func (m Model) SelectUp() Model {
	n := len(m.filtered)
	if n == 0 {
		return m
	}
	if m.selected <= 0 {
		m.selected = n - 1
	} else {
		m.selected--
	}
	return m
}

// SelectDown moves the selection one row down, wrapping to the first row if at bottom.
func (m Model) SelectDown() Model {
	n := len(m.filtered)
	if n == 0 {
		return m
	}
	if m.selected >= n-1 {
		m.selected = 0
	} else {
		m.selected++
	}
	return m
}

// StartEdit begins editing the currently selected row.
// The edit buffer starts empty; the user types a fresh replacement value.
// A no-op if the entry is ReadOnly or no entry is selected.
func (m Model) StartEdit() Model {
	if len(m.filtered) == 0 {
		return m
	}
	entry := m.filtered[m.selected]
	if entry.ReadOnly {
		return m
	}
	m.editing = true
	m.editBuf = ""
	return m
}

// EditInput appends a character to the edit buffer.
// A no-op if not in editing mode.
func (m Model) EditInput(ch rune) Model {
	if !m.editing {
		return m
	}
	m.editBuf += string(ch)
	return m
}

// EditBackspace removes the last character from the edit buffer (UTF-8 safe).
// A no-op if not in editing mode.
func (m Model) EditBackspace() Model {
	if !m.editing {
		return m
	}
	runes := []rune(m.editBuf)
	if len(runes) > 0 {
		m.editBuf = string(runes[:len(runes)-1])
	}
	return m
}

// CommitEdit applies editBuf to the selected entry's Value and marks it Dirty.
// Exits edit mode.
func (m Model) CommitEdit() Model {
	if !m.editing || len(m.filtered) == 0 {
		m.editing = false
		m.editBuf = ""
		return m
	}

	idx := m.selected
	newVal := m.editBuf

	// Update in filtered list.
	filtered := make([]ConfigEntry, len(m.filtered))
	copy(filtered, m.filtered)
	filtered[idx].Value = newVal
	filtered[idx].Dirty = true
	m.filtered = filtered

	// Propagate change back to the canonical entries slice by key.
	key := filtered[idx].Key
	entries := make([]ConfigEntry, len(m.entries))
	copy(entries, m.entries)
	for i := range entries {
		if entries[i].Key == key {
			entries[i].Value = newVal
			entries[i].Dirty = true
			break
		}
	}
	m.entries = entries

	m.editing = false
	m.editBuf = ""
	return m
}

// CancelEdit discards the edit buffer without changing the entry value.
// Exits edit mode.
func (m Model) CancelEdit() Model {
	m.editing = false
	m.editBuf = ""
	return m
}

// SelectedEntry returns the currently selected ConfigEntry.
// Returns (zero, false) if the filtered list is empty or selection is out of bounds.
func (m Model) SelectedEntry() (ConfigEntry, bool) {
	if len(m.filtered) == 0 {
		return ConfigEntry{}, false
	}
	idx := m.selected
	if idx < 0 || idx >= len(m.filtered) {
		return ConfigEntry{}, false
	}
	return m.filtered[idx], true
}

// IsEditing reports whether the panel is in inline edit mode.
func (m Model) IsEditing() bool {
	return m.editing
}

// View renders the config panel at the given terminal dimensions.
// If width or height are zero/negative, defaults of 70x20 are used.
func (m Model) View(width, height int) string {
	if width <= 0 {
		width = 70
	}
	if height <= 0 {
		height = 20
	}
	return render(m, width, height)
}

// filterEntries returns entries whose Key, Value, or Description contain q (case-insensitive).
// If q is empty, all entries are returned (as a copy).
func filterEntries(entries []ConfigEntry, q string) []ConfigEntry {
	if q == "" {
		cp := make([]ConfigEntry, len(entries))
		copy(cp, entries)
		return cp
	}
	lower := strings.ToLower(q)
	var result []ConfigEntry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Key), lower) ||
			strings.Contains(strings.ToLower(e.Value), lower) ||
			strings.Contains(strings.ToLower(e.Description), lower) {
			result = append(result, e)
		}
	}
	return result
}

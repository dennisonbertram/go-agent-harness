package memory

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Index manages the MEMORY.md index file which lists all memory topic entries.
type Index struct {
	// Entries is the ordered list of index entries. Oldest first, newest last.
	Entries []IndexEntry
	// Path is the absolute path to the MEMORY.md file.
	Path string
}

// IndexEntry is a single row in the MEMORY.md index table.
type IndexEntry struct {
	Name        string
	Type        MemoryType
	Description string
	FilePath    string
}

// LoadIndex reads and parses MEMORY.md from path.
// If the file is empty or has no table rows, it returns an Index with no entries.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}

	idx := &Index{Path: path}

	if len(data) == 0 {
		return idx, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	inTable := false
	headerParsed := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Detect the table header row
		if strings.HasPrefix(trimmed, "| Name") && strings.Contains(trimmed, "| Type") {
			inTable = true
			headerParsed = false
			continue
		}
		// Detect the table separator row (e.g., |------|------|...)
		if inTable && !headerParsed && strings.HasPrefix(trimmed, "|---") {
			headerParsed = true
			continue
		}
		// Parse table data rows
		if inTable && headerParsed && strings.HasPrefix(trimmed, "|") && !strings.HasPrefix(trimmed, "|---") {
			entry, ok := parseTableRow(trimmed)
			if ok {
				idx.Entries = append(idx.Entries, entry)
			}
			continue
		}
		// End of table
		if inTable && headerParsed && !strings.HasPrefix(trimmed, "|") && trimmed != "" {
			inTable = false
		}
	}

	return idx, scanner.Err()
}

// parseTableRow parses a Markdown table row into an IndexEntry.
// Expected format: | Name | Type | Description | File |
func parseTableRow(line string) (IndexEntry, bool) {
	// Remove leading/trailing pipe characters and split
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	cols := strings.Split(line, "|")
	if len(cols) < 4 {
		return IndexEntry{}, false
	}
	name := strings.TrimSpace(cols[0])
	typStr := strings.TrimSpace(cols[1])
	desc := strings.TrimSpace(cols[2])
	file := strings.TrimSpace(cols[3])

	if name == "" {
		return IndexEntry{}, false
	}

	return IndexEntry{
		Name:        name,
		Type:        MemoryType(typStr),
		Description: desc,
		FilePath:    file,
	}, true
}

// serialize renders the index as a Markdown string.
func (idx *Index) serialize() string {
	var sb strings.Builder
	sb.WriteString("# Memory Index\n\n")
	sb.WriteString("| Name | Type | Description | File |\n")
	sb.WriteString("|------|------|-------------|------|\n")
	for _, e := range idx.Entries {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", e.Name, e.Type, e.Description, e.FilePath))
	}
	return sb.String()
}

// SaveIndex writes MEMORY.md respecting the configured line and byte caps.
// It trims oldest entries first if needed before writing.
func (idx *Index) SaveIndex(cfg MemoryConfig) error {
	idx.TrimToFit(cfg)
	data := idx.serialize()
	return os.WriteFile(idx.Path, []byte(data), 0644)
}

// AddEntry appends a new entry to the index, enforcing caps by trimming oldest
// entries if the index would exceed its configured limits after adding.
func (idx *Index) AddEntry(entry IndexEntry, cfg MemoryConfig) error {
	idx.Entries = append(idx.Entries, entry)
	if idx.IsOverCap(cfg) {
		idx.TrimToFit(cfg)
	}
	return nil
}

// RemoveEntry removes the entry with the given name.
// If no entry has that name, it is a no-op (not an error).
func (idx *Index) RemoveEntry(name string) error {
	filtered := idx.Entries[:0]
	for _, e := range idx.Entries {
		if e.Name != name {
			filtered = append(filtered, e)
		}
	}
	idx.Entries = filtered
	return nil
}

// EntriesByType returns the subset of entries whose Type matches t.
func (idx *Index) EntriesByType(t MemoryType) []IndexEntry {
	var result []IndexEntry
	for _, e := range idx.Entries {
		if e.Type == t {
			result = append(result, e)
		}
	}
	return result
}

// IsOverCap reports whether the serialized index exceeds the configured line
// or byte limits.
func (idx *Index) IsOverCap(cfg MemoryConfig) bool {
	data := idx.serialize()
	lines := strings.Count(data, "\n")
	if cfg.IndexMaxLines > 0 && lines > cfg.IndexMaxLines {
		return true
	}
	if cfg.IndexMaxBytes > 0 && len(data) > cfg.IndexMaxBytes {
		return true
	}
	return false
}

// TrimToFit removes the oldest entries (those earliest in the slice) until the
// index no longer exceeds its caps. It returns the number of entries removed.
func (idx *Index) TrimToFit(cfg MemoryConfig) int {
	removed := 0
	for idx.IsOverCap(cfg) && len(idx.Entries) > 0 {
		idx.Entries = idx.Entries[1:]
		removed++
	}
	return removed
}

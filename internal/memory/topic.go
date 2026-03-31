package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TopicFile represents an individual memory topic file with YAML frontmatter.
type TopicFile struct {
	Entry   MemoryEntry
	Content string
}

// LoadTopicFile reads a topic file at path, parsing optional YAML frontmatter
// delimited by "---" lines. If no frontmatter is present, the entire file
// content is stored in Content and Entry is left at zero values.
func LoadTopicFile(path string) (*TopicFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read topic file: %w", err)
	}

	tf := &TopicFile{}
	tf.Entry.FilePath = path

	content := string(data)

	// Check for YAML frontmatter: file must start with "---\n"
	if !strings.HasPrefix(content, "---\n") {
		tf.Content = content
		return tf, nil
	}

	// Find the closing "---"
	rest := content[4:] // skip leading "---\n"
	end := strings.Index(rest, "\n---\n")
	if end == -1 {
		// No closing delimiter — treat whole file as content
		tf.Content = content
		return tf, nil
	}

	frontmatter := rest[:end]
	body := rest[end+5:] // skip "\n---\n"

	if err := yaml.Unmarshal([]byte(frontmatter), &tf.Entry); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	tf.Content = body

	return tf, nil
}

// SaveTopicFile writes a topic file with YAML frontmatter to tf.Entry.FilePath.
func SaveTopicFile(topic *TopicFile) error {
	frontmatter, err := yaml.Marshal(&topic.Entry)
	if err != nil {
		return fmt.Errorf("marshal frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(frontmatter)
	sb.WriteString("---\n")
	sb.WriteString(topic.Content)

	return os.WriteFile(topic.Entry.FilePath, []byte(sb.String()), 0644)
}

// ListTopicFiles returns the absolute paths of all .md files in dir.
// It does not recurse into subdirectories.
func ListTopicFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) == ".md" {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths, nil
}

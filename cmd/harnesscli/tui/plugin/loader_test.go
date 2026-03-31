package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	. "go-agent-harness/cmd/harnesscli/tui/plugin"
)

// writeJSON writes a JSON-encoded value to a file in dir with the given filename.
func writeJSON(t *testing.T, dir, filename string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), b, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

// BT-001: When the plugins directory does not exist, LoadPlugins returns (nil, nil).
func TestLoadPlugins_DirectoryNotExist(t *testing.T) {
	plugins, errs := LoadPlugins("/nonexistent/path/that/does/not/exist")
	if plugins != nil {
		t.Errorf("expected nil plugins, got %v", plugins)
	}
	if errs != nil {
		t.Errorf("expected nil errors, got %v", errs)
	}
}

// BT-002: Given a valid deploy.json with handler=bash, name=deploy, command=./deploy.sh,
// LoadPlugins returns one PluginDef with all fields populated.
func TestLoadPlugins_ValidBashPlugin(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "deploy.json", map[string]any{
		"name":        "deploy",
		"description": "Deploy the app",
		"handler":     "bash",
		"command":     "./deploy.sh",
	})

	plugins, errs := LoadPlugins(dir)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	p := plugins[0]
	if p.Name != "deploy" {
		t.Errorf("Name: got %q, want %q", p.Name, "deploy")
	}
	if p.Description != "Deploy the app" {
		t.Errorf("Description: got %q, want %q", p.Description, "Deploy the app")
	}
	if p.Handler != HandlerBash {
		t.Errorf("Handler: got %q, want %q", p.Handler, HandlerBash)
	}
	if p.Command != "./deploy.sh" {
		t.Errorf("Command: got %q, want %q", p.Command, "./deploy.sh")
	}
}

// BT-003: Given a JSON file with missing name field, the file is skipped and one error
// describes the file path and missing field.
func TestLoadPlugins_MissingName(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "noname.json", map[string]any{
		"handler": "bash",
		"command": "./run.sh",
	})

	plugins, errs := LoadPlugins(dir)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	errMsg := errs[0].Error()
	if !containsAll(errMsg, "noname.json") {
		t.Errorf("error %q should mention filename noname.json", errMsg)
	}
}

// BT-004: Given a plugin with handler=lua (unknown type), that plugin is skipped
// and an error describes the invalid handler type.
func TestLoadPlugins_UnknownHandler(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "lua-plugin.json", map[string]any{
		"name":    "myplugin",
		"handler": "lua",
	})

	plugins, errs := LoadPlugins(dir)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	errMsg := errs[0].Error()
	if !containsAll(errMsg, "lua-plugin.json") {
		t.Errorf("error %q should mention filename lua-plugin.json", errMsg)
	}
}

// BT-005: Given a plugin name with a space or slash, that plugin is skipped with an error.
func TestLoadPlugins_NameWithSpaceOrSlash(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "space-name.json", map[string]any{
		"name":    "my plugin",
		"handler": "bash",
		"command": "./run.sh",
	})
	writeJSON(t, dir, "slash-name.json", map[string]any{
		"name":    "my/plugin",
		"handler": "bash",
		"command": "./run.sh",
	})

	plugins, errs := LoadPlugins(dir)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

// BT-006: Given 3 valid files and 1 invalid file, 3 PluginDefs and 1 error are returned.
func TestLoadPlugins_MixedValidInvalid(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "a.json", map[string]any{
		"name":    "alpha",
		"handler": "bash",
		"command": "./a.sh",
	})
	writeJSON(t, dir, "b.json", map[string]any{
		"name":            "beta",
		"handler":         "prompt",
		"prompt_template": "summarize {{.Input}}",
	})
	writeJSON(t, dir, "c.json", map[string]any{
		"name":    "gamma",
		"handler": "bash",
		"command": "./c.sh",
	})
	writeJSON(t, dir, "bad.json", map[string]any{
		"name":    "Bad Plugin",
		"handler": "bash",
		"command": "./bad.sh",
	})

	plugins, errs := LoadPlugins(dir)
	if len(plugins) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

// BT-007: Given a plugin name starting with uppercase or number, that plugin is rejected.
func TestLoadPlugins_NameStartsWithUppercaseOrNumber(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "upper.json", map[string]any{
		"name":    "Deploy",
		"handler": "bash",
		"command": "./deploy.sh",
	})
	writeJSON(t, dir, "number.json", map[string]any{
		"name":    "1deploy",
		"handler": "bash",
		"command": "./deploy.sh",
	})

	plugins, errs := LoadPlugins(dir)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

// BT-008: Given a bash plugin with empty Command field, that plugin is rejected.
func TestLoadPlugins_BashWithEmptyCommand(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "no-cmd.json", map[string]any{
		"name":    "nocmd",
		"handler": "bash",
		"command": "",
	})

	plugins, errs := LoadPlugins(dir)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

// --- Table-driven regression tests ---

func TestValidate_AllRules(t *testing.T) {
	tests := []struct {
		name    string
		def     PluginDef
		wantErr bool
		errHint string // substring expected in error message
	}{
		{
			name:    "missing name",
			def:     PluginDef{Name: "", Handler: HandlerBash, Command: "./run.sh"},
			wantErr: true,
			errHint: "name",
		},
		{
			name:    "empty name explicit",
			def:     PluginDef{Name: "", Handler: HandlerBash, Command: "./run.sh"},
			wantErr: true,
			errHint: "name",
		},
		{
			name:    "name with space",
			def:     PluginDef{Name: "my plugin", Handler: HandlerBash, Command: "./run.sh"},
			wantErr: true,
			errHint: "name",
		},
		{
			name:    "name with slash",
			def:     PluginDef{Name: "my/plugin", Handler: HandlerBash, Command: "./run.sh"},
			wantErr: true,
			errHint: "name",
		},
		{
			name:    "name starting with number",
			def:     PluginDef{Name: "1plugin", Handler: HandlerBash, Command: "./run.sh"},
			wantErr: true,
			errHint: "name",
		},
		{
			name:    "name with uppercase",
			def:     PluginDef{Name: "MyPlugin", Handler: HandlerBash, Command: "./run.sh"},
			wantErr: true,
			errHint: "name",
		},
		{
			name:    "valid name only lowercase",
			def:     PluginDef{Name: "my-plugin", Handler: HandlerBash, Command: "./run.sh"},
			wantErr: false,
		},
		{
			name:    "missing handler",
			def:     PluginDef{Name: "myplugin", Handler: ""},
			wantErr: true,
			errHint: "handler",
		},
		{
			name:    "unknown handler",
			def:     PluginDef{Name: "myplugin", Handler: "lua"},
			wantErr: true,
			errHint: "handler",
		},
		{
			name:    "bash without command",
			def:     PluginDef{Name: "myplugin", Handler: HandlerBash, Command: ""},
			wantErr: true,
			errHint: "command",
		},
		{
			name:    "prompt without template",
			def:     PluginDef{Name: "myplugin", Handler: HandlerPrompt, PromptTemplate: ""},
			wantErr: true,
			errHint: "prompt_template",
		},
		{
			name:    "valid bash plugin",
			def:     PluginDef{Name: "deploy", Handler: HandlerBash, Command: "./deploy.sh"},
			wantErr: false,
		},
		{
			name:    "valid prompt plugin",
			def:     PluginDef{Name: "summarize", Handler: HandlerPrompt, PromptTemplate: "summarize {{.Input}}"},
			wantErr: false,
		},
		{
			name:    "name with digits after first char is valid",
			def:     PluginDef{Name: "deploy2", Handler: HandlerBash, Command: "./deploy.sh"},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.def.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
			if tc.wantErr && err != nil && tc.errHint != "" {
				if !containsAll(err.Error(), tc.errHint) {
					t.Errorf("error %q should contain %q", err.Error(), tc.errHint)
				}
			}
		})
	}
}

// Regression test: loading a valid prompt plugin populates PromptTemplate correctly.
func TestLoadPlugins_ValidPromptPlugin(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "summarize.json", map[string]any{
		"name":            "summarize",
		"description":     "Summarize text",
		"handler":         "prompt",
		"prompt_template": "Please summarize: {{.Input}}",
	})

	plugins, errs := LoadPlugins(dir)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	p := plugins[0]
	if p.Handler != HandlerPrompt {
		t.Errorf("Handler: got %q, want %q", p.Handler, HandlerPrompt)
	}
	if p.PromptTemplate != "Please summarize: {{.Input}}" {
		t.Errorf("PromptTemplate: got %q", p.PromptTemplate)
	}
}

// Regression test: non-JSON files are not parsed (only *.json).
func TestLoadPlugins_NonJSONFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	// write a valid plugin
	writeJSON(t, dir, "valid.json", map[string]any{
		"name":    "valid",
		"handler": "bash",
		"command": "./run.sh",
	})
	// write a non-json file that would fail to parse
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# Notes"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, errs := LoadPlugins(dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}
}

// Regression test: malformed JSON returns an error mentioning the filename.
func TestLoadPlugins_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, errs := LoadPlugins(dir)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !containsAll(errs[0].Error(), "broken.json") {
		t.Errorf("error %q should mention broken.json", errs[0].Error())
	}
}

// Regression test: empty directory returns (nil, nil).
func TestLoadPlugins_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	plugins, errs := LoadPlugins(dir)
	if plugins != nil {
		t.Errorf("expected nil plugins, got %v", plugins)
	}
	if errs != nil {
		t.Errorf("expected nil errors, got %v", errs)
	}
}

// containsAll checks if s contains all substrings.
func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

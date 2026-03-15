package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkspacePath(t *testing.T) {
	root := "/workspace"
	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
		wantPath    string
	}{
		{
			name:    "empty root returns error",
			path:    "foo.txt",
			wantErr: true,
		},
		{
			name:     "simple relative path",
			path:     "foo.txt",
			wantPath: "/workspace/foo.txt",
		},
		{
			name:     "nested relative path",
			path:     "dir/sub/file.go",
			wantPath: "/workspace/dir/sub/file.go",
		},
		{
			name:     "empty path returns workspace root",
			path:     "",
			wantPath: "/workspace",
		},
		{
			name:     "dot path returns workspace root",
			path:     ".",
			wantPath: "/workspace",
		},
		{
			name:     "absolute path passes through",
			path:     "/etc/nginx/nginx.conf",
			wantPath: "/etc/nginx/nginx.conf",
		},
		{
			name:     "absolute path cleaned",
			path:     "/var/log/../log/nginx/access.log",
			wantPath: "/var/log/nginx/access.log",
		},
		{
			name:        "path escaping workspace",
			path:        "../../etc/passwd",
			wantErr:     true,
			errContains: "escapes workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r string
			var err error
			if tt.name == "empty root returns error" {
				r, err = ResolveWorkspacePath("", tt.path)
			} else {
				r, err = ResolveWorkspacePath(root, tt.path)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got path %q", r)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := filepath.Clean(tt.wantPath)
			if r != want {
				t.Fatalf("got %q, want %q", r, want)
			}
		})
	}
}

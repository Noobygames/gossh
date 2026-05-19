package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Unanchored: matches any component
		{"*.log", "debug.log", true},
		{"*.log", "src/debug.log", true},
		{"*.log", "src/sub/debug.log", true},
		{"*.log", "src/debug.go", false},
		{"node_modules", "node_modules/pkg", true},
		{"node_modules", "a/node_modules/pkg", true},
		{"node_modules", "a/not_node_modules/pkg", false},

		// Trailing slash: same as without (directory-only marker, type not enforced)
		{"dist/", "dist/main.js", true},
		{"dist/", "a/dist/main.js", true},

		// Anchored: leading slash matches from root.
		// Pattern "/vendor" matches the "vendor" directory entry itself;
		// subdirectory contents are excluded via SkipDir in the walk,
		// not by the pattern matching them directly.
		{"/vendor", "vendor", true},
		{"/vendor", "a/vendor", false},

		// Anchored: slash in middle implies root-relative
		{"src/*.go", "src/main.go", true},
		{"src/*.go", "other/src/main.go", false},
		{"src/*.go", "src/sub/main.go", false},

		// ** anywhere
		{"**/node_modules", "node_modules", true},
		{"**/node_modules", "a/node_modules", true},
		{"**/node_modules", "a/b/node_modules", true},
		{"**/node_modules", "a/b/node_modulesX", false},
		{"src/**/*.go", "src/main.go", true},
		{"src/**/*.go", "src/sub/main.go", true},
		{"src/**/*.go", "src/a/b/main.go", true},
		{"src/**/*.go", "other/main.go", false},
		{"src/**", "src/anything", true},
		{"src/**", "src", true},
	}
	for _, tt := range tests {
		got := matchPattern(tt.path, tt.pattern)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
		}
	}
}

func TestIsExcludedGitignoreStyle(t *testing.T) {
	tests := []struct {
		desc     string
		path     string
		excludes []string
		want     bool
	}{
		{
			desc:     "simple name match",
			path:     ".git/config",
			excludes: []string{".git"},
			want:     true,
		},
		{
			desc:     "glob match",
			path:     "logs/app.log",
			excludes: []string{"*.log"},
			want:     true,
		},
		{
			desc:     "negation overrides earlier match",
			path:     "important.log",
			excludes: []string{"*.log", "!important.log"},
			want:     false,
		},
		{
			desc:     "comment lines ignored",
			path:     "file.txt",
			excludes: []string{"# this is a comment", "*.txt"},
			want:     true,
		},
		{
			desc:     "empty lines ignored",
			path:     "file.txt",
			excludes: []string{"", "*.txt"},
			want:     true,
		},
		{
			desc:     "last match wins with multiple patterns",
			path:     "src/debug.log",
			excludes: []string{"*.log", "!src/*.log", "*.log"},
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if got := isExcluded(tt.path, tt.excludes); got != tt.want {
				t.Errorf("isExcluded(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	content := `
server: user@host.example.com
remote-dir: ~/configs
excludes:
  - "*.bak"
  - "dist/"
`
	must(t, os.WriteFile(filepath.Join(dir, ".gossh.yml"), []byte(content), 0644))

	orig, err := os.Getwd()
	must(t, err)
	must(t, os.Chdir(dir))
	defer os.Chdir(orig) //nolint:errcheck

	cfg, err := loadConfig()
	must(t, err)

	if cfg.Server != "user@host.example.com" {
		t.Errorf("Server = %q, want %q", cfg.Server, "user@host.example.com")
	}
	if cfg.RemoteDir != "~/configs" {
		t.Errorf("RemoteDir = %q, want %q", cfg.RemoteDir, "~/configs")
	}
	if len(cfg.Excludes) != 2 || cfg.Excludes[0] != "*.bak" || cfg.Excludes[1] != "dist/" {
		t.Errorf("Excludes = %v, want [*.bak dist/]", cfg.Excludes)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	must(t, err)
	must(t, os.Chdir(dir))
	defer os.Chdir(orig) //nolint:errcheck

	// Redirect HOME so the fallback ~/.gossh.yml is not found either.
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	cfg, err := loadConfig()
	must(t, err)

	if cfg.Server != "" || cfg.RemoteDir != "" || len(cfg.Excludes) != 0 {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

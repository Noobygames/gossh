package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noobygames/gossh/pkg/config"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.log", "debug.log", true},
		{"*.log", "src/debug.log", true},
		{"*.log", "src/sub/debug.log", true},
		{"*.log", "src/debug.go", false},
		{"node_modules", "node_modules/pkg", true},
		{"node_modules", "a/node_modules/pkg", true},
		{"node_modules", "a/not_node_modules/pkg", false},
		{"dist/", "dist/main.js", true},
		{"dist/", "a/dist/main.js", true},
		{"/vendor", "vendor", true},
		{"/vendor", "a/vendor", false},
		{"src/*.go", "src/main.go", true},
		{"src/*.go", "other/src/main.go", false},
		{"src/*.go", "src/sub/main.go", false},
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
		got := config.MatchPattern(tt.path, tt.pattern)
		if got != tt.want {
			t.Errorf("MatchPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
		}
	}
}

func TestIsExcludedGitignoreStyle(t *testing.T) {
	type tc struct {
		desc     string
		path     string
		excludes []string
		want     bool
	}
	tests := []tc{
		{"simple name match", ".git/config", []string{".git"}, true},
		{"glob match", "logs/app.log", []string{"*.log"}, true},
		{"negation overrides earlier match", "important.log", []string{"*.log", "!important.log"}, false},
		{"comment lines ignored", "file.txt", []string{"# comment", "*.txt"}, true},
		{"empty lines ignored", "file.txt", []string{"", "*.txt"}, true},
		{"last match wins", "src/debug.log", []string{"*.log", "!src/*.log", "*.log"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			excluded := false
			for _, p := range tt.excludes {
				if p == "" || p[0] == '#' {
					continue
				}
				neg := p[0] == '!'
				pat := p
				if neg {
					pat = p[1:]
				}
				if config.MatchPattern(tt.path, pat) {
					excluded = !neg
				}
			}
			if excluded != tt.want {
				t.Errorf("path %q excludes %v: got %v, want %v", tt.path, tt.excludes, excluded, tt.want)
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

	cfg, err := config.Load()
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

	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	cfg, err := config.Load()
	must(t, err)

	if cfg.Server != "" || cfg.RemoteDir != "" || len(cfg.Excludes) != 0 {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

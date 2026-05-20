package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noobygames/gossh/pkg/config"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"glob any segment", "*.log", "debug.log", true},
		{"glob in subdirectory", "*.log", "src/debug.log", true},
		{"glob in nested subdirectory", "*.log", "src/sub/debug.log", true},
		{"glob no match", "*.log", "src/debug.go", false},
		{"name any segment", "node_modules", "node_modules/pkg", true},
		{"name nested segment", "node_modules", "a/node_modules/pkg", true},
		{"name partial no match", "node_modules", "a/not_node_modules/pkg", false},
		{"trailing slash matches dir segment", "dist/", "dist/main.js", true},
		{"trailing slash nested", "dist/", "a/dist/main.js", true},
		{"root anchor exact", "/vendor", "vendor", true},
		{"root anchor no nested match", "/vendor", "a/vendor", false},
		{"anchored glob", "src/*.go", "src/main.go", true},
		{"anchored glob other dir", "src/*.go", "other/src/main.go", false},
		{"anchored glob subdirectory", "src/*.go", "src/sub/main.go", false},
		{"double-star prefix", "**/node_modules", "node_modules", true},
		{"double-star single level", "**/node_modules", "a/node_modules", true},
		{"double-star nested", "**/node_modules", "a/b/node_modules", true},
		{"double-star partial no match", "**/node_modules", "a/b/node_modulesX", false},
		{"double-star in middle shallow", "src/**/*.go", "src/main.go", true},
		{"double-star in middle deep", "src/**/*.go", "src/sub/main.go", true},
		{"double-star in middle nested", "src/**/*.go", "src/a/b/main.go", true},
		{"double-star in middle other dir", "src/**/*.go", "other/main.go", false},
		{"double-star suffix", "src/**", "src/anything", true},
		{"double-star suffix exact", "src/**", "src", true},
		// Malformed patterns must not panic and must not match.
		{"malformed bracket no match", "[abc", "foo.go", false},
		{"malformed bracket anchored no match", "/[abc", "foo.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, config.MatchPattern(tt.path, tt.pattern))
		})
	}
}

func TestGlobMatchMalformedPattern(t *testing.T) {
	assert.False(t, config.GlobMatch("[abc", "foo.go"), "malformed pattern should not match")
	assert.False(t, config.GlobMatch("[abc", ""), "malformed pattern should not match empty string")
}

func TestIsExcludedGitignoreStyle(t *testing.T) {
	tests := []struct {
		desc     string
		path     string
		excludes []string
		want     bool
	}{
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
			assert.Equal(t, tt.want, excluded)
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gossh.yml"), []byte(content), 0644))

	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(orig) //nolint:errcheck

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "user@host.example.com", cfg.Server)
	assert.Equal(t, "~/configs", cfg.RemoteDir)
	assert.Equal(t, []string{"*.bak", "dist/"}, cfg.Excludes)
}

func TestLoadConfigMissing(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(orig) //nolint:errcheck

	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Empty(t, cfg.Server)
	assert.Empty(t, cfg.RemoteDir)
	assert.Empty(t, cfg.Excludes)
}

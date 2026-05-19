package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	Server    string   `yaml:"server"`
	RemoteDir string   `yaml:"remote-dir"`
	Excludes  []string `yaml:"excludes"`
}

// loadConfig searches for .gomvtossh.yml in the current directory, then in
// the user's home directory. Returns an empty config if no file is found.
func loadConfig() (fileConfig, error) {
	candidates := []string{".gomvtossh.yml"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".gomvtossh.yml"))
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fileConfig{}, fmt.Errorf("read config %s: %w", p, err)
		}
		var cfg fileConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fileConfig{}, fmt.Errorf("parse config %s: %w", p, err)
		}
		return cfg, nil
	}
	return fileConfig{}, nil
}

// matchPattern reports whether the slash-separated relPath matches a single
// gitignore-style pattern.
//
//   - Patterns without a slash match any path component by name (glob).
//   - Patterns with a slash are anchored to the root (glob from root).
//   - Leading "/" is stripped before matching.
//   - Trailing "/" is stripped (directory-only marker; type is not checked here).
//   - "**" matches any sequence of path segments.
func matchPattern(relPath, pattern string) bool {
	pattern = strings.TrimSuffix(pattern, "/")

	// Anchored: pattern contains "/" (other than the now-stripped trailing one)
	// or starts with "/".
	raw := strings.TrimPrefix(pattern, "/")
	anchored := strings.HasPrefix(pattern, "/") || strings.Contains(raw, "/")
	pattern = raw

	if anchored {
		return globMatch(pattern, relPath)
	}
	// Unanchored: match against each individual path component.
	for comp := range strings.SplitSeq(relPath, "/") {
		if ok, _ := path.Match(pattern, comp); ok {
			return true
		}
	}
	return false
}

// globMatch matches pattern against s with support for "**".
// Recurses to handle multiple "**" segments.
func globMatch(pattern, s string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := path.Match(pattern, s)
		return ok
	}

	before, after, _ := strings.Cut(pattern, "**")
	prefix := strings.TrimSuffix(before, "/")
	rest := strings.TrimPrefix(after, "/")

	// prefix must match the start of s
	if prefix != "" {
		if s != prefix && !strings.HasPrefix(s, prefix+"/") {
			return false
		}
		if s == prefix {
			return rest == ""
		}
		s = s[len(prefix)+1:] // strip "prefix/"
	}

	if rest == "" {
		return true // "**" at the end matches everything
	}

	// Try matching rest (which may itself contain "**") against every suffix.
	segs := strings.Split(s, "/")
	for i := range segs {
		if globMatch(rest, strings.Join(segs[i:], "/")) {
			return true
		}
	}
	return false
}

// firstNonEmpty returns the first non-empty string from vals.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

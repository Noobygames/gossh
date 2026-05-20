package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds values from a .gossh.yml configuration file.
type Config struct {
	Server    string   `yaml:"server"`
	RemoteDir string   `yaml:"remote-dir"`
	Excludes  []string `yaml:"excludes"`
}

// Load reads the first .gossh.yml found in the current directory or the
// user's home directory. Returns an empty Config if no file is present.
func Load() (Config, error) {
	candidates := []string{".gossh.yml"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".gossh.yml"))
	}
	for _, p := range candidates {
		cfg, ok, err := tryLoadConfig(p)
		if err != nil {
			return Config{}, err
		}
		if ok {
			return cfg, nil
		}
	}
	return Config{}, nil
}

// tryLoadConfig attempts to read and parse a single config file.
// Returns (cfg, true, nil) on success, (zero, false, nil) if the file does not exist.
func tryLoadConfig(p string) (Config, bool, error) {
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, fmt.Errorf("read config %s: %w", p, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, &ParseError{Path: p, Err: err}
	}
	return cfg, true, nil
}

// MatchPattern reports whether the slash-separated relPath matches a single
// gitignore-style pattern. Trailing "/" (dir-only marker), leading "/" (root
// anchor), "**" (multi-segment wildcard), and component glob are all supported.
func MatchPattern(relPath, pattern string) bool {
	pattern = strings.TrimSuffix(pattern, "/")
	raw := strings.TrimPrefix(pattern, "/")
	anchored := strings.HasPrefix(pattern, "/") || strings.Contains(raw, "/")
	pattern = raw
	if anchored {
		return GlobMatch(pattern, relPath)
	}
	for comp := range strings.SplitSeq(relPath, "/") {
		ok, err := path.Match(pattern, comp)
		if err != nil {
			// path.ErrBadPattern: malformed pattern from user config (e.g. "[abc").
			// The pattern is invalid for every component, so no match is possible.
			return false
		}
		if ok {
			return true
		}
	}
	return false
}

// GlobMatch matches pattern against s with support for "**".
func GlobMatch(pattern, s string) bool {
	if !strings.Contains(pattern, "**") {
		ok, err := path.Match(pattern, s)
		if err != nil {
			// path.ErrBadPattern: malformed pattern; treat as no match.
			return false
		}
		return ok
	}
	before, after, _ := strings.Cut(pattern, "**")
	prefix := strings.TrimSuffix(before, "/")
	rest := strings.TrimPrefix(after, "/")
	if prefix != "" {
		if s != prefix && !strings.HasPrefix(s, prefix+"/") {
			return false
		}
		if s == prefix {
			return rest == ""
		}
		s = s[len(prefix)+1:]
	}
	if rest == "" {
		return true
	}
	segs := strings.Split(s, "/")
	for i := range segs {
		if GlobMatch(rest, strings.Join(segs[i:], "/")) {
			return true
		}
	}
	return false
}

// FirstNonEmpty returns the first non-empty string from vals.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryLoadConfig_NotExist(t *testing.T) {
	cfg, ok, err := tryLoadConfig(filepath.Join(t.TempDir(), "no-such.yml"))

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, cfg.Server)
}

func TestTryLoadConfig_Valid(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".gossh.yml")
	require.NoError(t, os.WriteFile(f, []byte("server: user@host\nremote-dir: ~/k8s\n"), 0644))

	cfg, ok, err := tryLoadConfig(f)

	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "user@host", cfg.Server)
	assert.Equal(t, "~/k8s", cfg.RemoteDir)
}

func TestTryLoadConfig_InvalidYAML(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".gossh.yml")
	require.NoError(t, os.WriteFile(f, []byte(":\tinvalid::\n"), 0644))

	_, _, err := tryLoadConfig(f)

	require.Error(t, err)
	var parseErr *ParseError
	require.ErrorAs(t, err, &parseErr)
	assert.Equal(t, f, parseErr.Path)
	assert.NotNil(t, parseErr.Unwrap())
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want string
	}{
		{"first non-empty wins", []string{"a", "b"}, "a"},
		{"skips leading empty", []string{"", "b"}, "b"},
		{"skips multiple empty", []string{"", "", "c"}, "c"},
		{"all empty returns empty", []string{"", ""}, ""},
		{"nil returns empty", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FirstNonEmpty(tt.vals...))
		})
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRemotePath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "home prefix with subpath",
			input: filepath.Join(home, "kubernetes", "foo.yml"),
			want:  "~/kubernetes/foo.yml",
		},
		{
			name:  "home prefix with nested subpath",
			input: filepath.Join(home, "kubernetes", "raid-assignments", "sealed-secret.yml"),
			want:  "~/kubernetes/raid-assignments/sealed-secret.yml",
		},
		{
			name:  "exact home dir",
			input: home,
			want:  "~",
		},
		{
			name:  "already tilde path passes through",
			input: "~/kubernetes/foo.yml",
			want:  "~/kubernetes/foo.yml",
		},
		{
			name:  "relative path passes through",
			input: "./local/path",
			want:  "./local/path",
		},
		{
			name:  "unrelated absolute path passes through",
			input: "/etc/hosts",
			want:  "/etc/hosts",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeRemotePath(tt.input))
		})
	}
}

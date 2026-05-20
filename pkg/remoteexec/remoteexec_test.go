package remoteexec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ExtractGosshFlags ---

func TestExtractGosshFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantServer   string
		wantIdentity string
		wantRest     []string
	}{
		{
			name:     "no flags",
			args:     []string{"apply", "-f", "manifest.yaml"},
			wantRest: []string{"apply", "-f", "manifest.yaml"},
		},
		{
			name:         "server space-separated",
			args:         []string{"-server", "user@host", "get", "pods"},
			wantServer:   "user@host",
			wantRest:     []string{"get", "pods"},
		},
		{
			name:         "server equals",
			args:         []string{"-server=user@host", "get", "pods"},
			wantServer:   "user@host",
			wantRest:     []string{"get", "pods"},
		},
		{
			name:         "double-dash server equals",
			args:         []string{"--server=user@host", "get"},
			wantServer:   "user@host",
			wantRest:     []string{"get"},
		},
		{
			name:         "identity space-separated",
			args:         []string{"-identity", "/home/user/.ssh/id_rsa", "get"},
			wantIdentity: "/home/user/.ssh/id_rsa",
			wantRest:     []string{"get"},
		},
		{
			name:         "both flags then rest",
			args:         []string{"-server", "u@h", "-identity", "key", "apply"},
			wantServer:   "u@h",
			wantIdentity: "key",
			wantRest:     []string{"apply"},
		},
		{
			name:     "server flag at end with no value",
			args:     []string{"-server"},
			wantRest: nil,
		},
		{
			name:     "empty args",
			args:     []string{},
			wantRest: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, identity, rest := ExtractGosshFlags(tt.args)
			assert.Equal(t, tt.wantServer, server)
			assert.Equal(t, tt.wantIdentity, identity)
			assert.Equal(t, tt.wantRest, rest)
		})
	}
}

// --- substituteArg ---

func TestSubstituteArg(t *testing.T) {
	mapping := map[string]string{
		"./local.yaml": "/tmp/gossh-abc/local.yaml",
		"./local-dir":  "/tmp/gossh-abc/local-dir",
	}
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{"positional match", "./local.yaml", "/tmp/gossh-abc/local.yaml"},
		{"dir match", "./local-dir", "/tmp/gossh-abc/local-dir"},
		{"no match passthrough", "unknown.yaml", "unknown.yaml"},
		{"flag equals match", "-f=./local.yaml", "-f=/tmp/gossh-abc/local.yaml"},
		{"flag equals no match", "-f=unknown.yaml", "-f=unknown.yaml"},
		{"flag without value", "-f", "-f"},
		{"flag equals no local path", "--dry-run=client", "--dry-run=client"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, substituteArg(tt.arg, mapping))
		})
	}
}

// --- buildUpload ---

func TestBuildUpload_NotLocalPath(t *testing.T) {
	_, ok, err := buildUpload("https://example.com/file", "/tmp/dir")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestBuildUpload_MissingFile(t *testing.T) {
	_, ok, err := buildUpload("/no/such/file", "/tmp/dir")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestBuildUpload_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "deploy.yaml")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0644))

	u, ok, err := buildUpload(f, "/tmp/gossh-abc")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, f, u.localPath)
	assert.Equal(t, "/tmp/gossh-abc/deploy.yaml", u.remotePath)
	assert.False(t, u.isDir)
}

func TestBuildUpload_Dir(t *testing.T) {
	dir := t.TempDir()

	u, ok, err := buildUpload(dir, "/tmp/gossh-abc")
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, u.isDir)
	assert.Equal(t, "/tmp/gossh-abc/"+filepath.Base(dir), u.remotePath)
}

// --- buildRemoteCommand ---

func TestBuildRemoteCommand(t *testing.T) {
	tests := []struct {
		name string
		tool string
		args []string
		want string
	}{
		{"simple args", "kubectl", []string{"get", "pods"}, "kubectl 'get' 'pods'"},
		{"flag with path", "kubectl", []string{"apply", "-f", "/tmp/manifest.yaml"}, "kubectl 'apply' '-f' '/tmp/manifest.yaml'"},
		{"path with spaces", "helm", []string{"install", "my-release", "path with spaces"}, "helm 'install' 'my-release' 'path with spaces'"},
		{"single quote in arg", "tool", []string{"it's"}, "tool 'it'\\''s'"},
		{"no args", "kubectl", []string{}, "kubectl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildRemoteCommand(tt.tool, tt.args))
		})
	}
}

// --- shellQuote ---

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple string", "simple", "'simple'"},
		{"string with space", "with space", "'with space'"},
		{"string with single quote", "it's", "'it'\\''s'"},
		{"tilde path", "~/kubernetes", "'~/kubernetes'"},
		{"empty string", "", "''"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shellQuote(tt.input))
		})
	}
}

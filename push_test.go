package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noobygames/gossh/pkg/config"
)

func TestRunPushTooManyArgs(t *testing.T) {
	err := runPush(context.Background(), "", "", "", false, nil,
		[]string{"a", "b", "c"}, config.Config{}, io.Discard)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many arguments")
}

func TestRunPushDryRunDefaultSourceDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("x"), 0644))

	var buf bytes.Buffer
	cfg := config.Config{Server: "user@host", RemoteDir: "~/k8s"}

	require.NoError(t, runPush(context.Background(), "", "", "", true, nil,
		[]string{dir}, cfg, &buf))

	assert.Contains(t, buf.String(), "deploy.yaml")
}

func TestRunPushDryRunExcludesDefaultPatterns(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("x"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0644))

	var buf bytes.Buffer
	cfg := config.Config{Server: "user@host"}

	require.NoError(t, runPush(context.Background(), "", "", "", true, nil,
		[]string{dir}, cfg, &buf))

	assert.Contains(t, buf.String(), "deploy.yaml")
	assert.NotContains(t, buf.String(), "HEAD", ".git content should be excluded by defaultExcludes")
}

func TestRunPushDryRunExtraExcludes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secret.yaml"), []byte("s"), 0644))

	var buf bytes.Buffer
	cfg := config.Config{Server: "user@host"}

	require.NoError(t, runPush(context.Background(), "", "", "", true,
		[]string{"secret.yaml"}, []string{dir}, cfg, &buf))

	assert.Contains(t, buf.String(), "sync  deploy.yaml")
	assert.NotContains(t, buf.String(), "sync  secret.yaml")
}

func TestRunPushDryRunServerFromConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.yaml"), []byte("x"), 0644))

	var buf bytes.Buffer
	cfg := config.Config{Server: "cfg@host", RemoteDir: "~/configured"}

	require.NoError(t, runPush(context.Background(), "", "", "", true, nil,
		[]string{dir}, cfg, &buf))

	// dry-run header should reference the config server
	assert.Contains(t, buf.String(), "cfg@host")
	assert.Contains(t, buf.String(), "~/configured")
}

func TestRunPushDryRunFlagServerOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.yaml"), []byte("x"), 0644))

	var buf bytes.Buffer
	cfg := config.Config{Server: "cfg@host"}

	require.NoError(t, runPush(context.Background(), "flag@host", "", "", true, nil,
		[]string{dir}, cfg, &buf))

	assert.Contains(t, buf.String(), "flag@host")
	assert.NotContains(t, buf.String(), "cfg@host")
}

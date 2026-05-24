package transfer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTarGz builds an in-memory gzip-compressed tar archive.
func makeTarGz(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     name,
			Mode:     0644,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return &buf
}

func readLocalFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func noPrompt(t *testing.T) ConflictPromptFn {
	return func(relPath string) (ConflictChoice, error) {
		assert.Fail(t, "unexpected conflict prompt", "path: %q", relPath)
		return 0, nil
	}
}

func fixedPrompt(c ConflictChoice) ConflictPromptFn {
	return func(_ string) (ConflictChoice, error) { return c, nil }
}

// --- isExcluded ---

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		excludes []string
		want     bool
	}{
		{"git dir matched by name", ".git/config", []string{".git"}, true},
		{"nested git dir matched by name", "src/.git/config", []string{".git"}, true},
		{"non-matching name", "main.go", []string{".git"}, false},
		{"exact filename match", "secret.yml", []string{"secret.yml"}, true},
		{"nested filename match", "nested/secret.yml", []string{"secret.yml"}, true},
		{"partial name does not match", "nosecret.yml", []string{"secret.yml"}, false},
		{"intermediate component matched", "a/b/c", []string{"b"}, true},
		{"empty excludes", "a/b/c", []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isExcluded(tt.path, tt.excludes))
		})
	}
}

// --- ListFiles ---

func TestListFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("c"), 0644))

	var buf bytes.Buffer
	require.NoError(t, ListFiles(&buf, dir, []string{".git"}))
	out := buf.String()

	assert.Contains(t, out, "a.txt")
	assert.Contains(t, out, "b.txt")
	assert.Contains(t, out, "SKIP")
	assert.Contains(t, out, ".git")
	assert.NotContains(t, out, "config", ".git/config should be hidden by SkipDir")
}

// --- ArchiveAndSend ---

func TestArchiveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"hello.txt": "hello", "sub/world.txt": "world"}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secret.yml"), []byte("secret"), 0644))

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		errCh <- ArchiveAndSend(pw, io.Discard, dir, []string{"secret.yml"})
	}()

	gr, err := gzip.NewReader(pr)
	require.NoError(t, err)
	tr := tar.NewReader(gr)

	got := map[string]string{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(tr)
		require.NoError(t, err)
		got[hdr.Name] = string(data)
	}
	require.NoError(t, <-errCh)

	for rel, want := range files {
		assert.Equal(t, want, got[rel], "file %q content mismatch", rel)
	}
	assert.NotContains(t, got, "secret.yml", "secret.yml should be excluded")
}

// --- Push dry-run ---

func TestPushDryRun(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("deploy"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0644))

	var buf bytes.Buffer
	opts := Options{
		SourceDir: dir,
		Server:    "user@host",
		RemoteDir: "~/k8s",
		DryRun:    true,
		Excludes:  []string{".git"},
		Out:       &buf,
	}
	require.NoError(t, Push(context.Background(), opts))

	out := buf.String()
	assert.Contains(t, out, "deploy.yaml")
	assert.NotContains(t, out, "HEAD", ".git/HEAD should be excluded")
}

// --- extractWithConflicts ---

func TestExtractNoConflicts(t *testing.T) {
	dir := t.TempDir()
	archive := makeTarGz(t, map[string]string{"./a.txt": "hello", "./sub/b.txt": "world"})
	opts := Options{SourceDir: dir, Out: io.Discard}

	require.NoError(t, extractWithConflicts(archive, opts, noPrompt(t)))

	assert.Equal(t, "hello", readLocalFile(t, filepath.Join(dir, "a.txt")))
	assert.Equal(t, "world", readLocalFile(t, filepath.Join(dir, "sub", "b.txt")))
}

func TestExtractConflictSkip(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local"), 0644))
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote", "./b.txt": "new"})
	opts := Options{SourceDir: dir, Out: io.Discard}

	require.NoError(t, extractWithConflicts(archive, opts, fixedPrompt(ChoiceSkip)))

	assert.Equal(t, "local", readLocalFile(t, filepath.Join(dir, "a.txt")), "a.txt should be unchanged")
	assert.Equal(t, "new", readLocalFile(t, filepath.Join(dir, "b.txt")), "b.txt should be written")
}

func TestExtractConflictSkipAll(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local-a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("local-b"), 0644))

	calls := 0
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote-a", "./b.txt": "remote-b", "./c.txt": "new-c"})
	opts := Options{SourceDir: dir, Out: io.Discard}
	require.NoError(t, extractWithConflicts(archive, opts, func(_ string) (ConflictChoice, error) {
		calls++
		return ChoiceSkipAll, nil
	}))

	assert.Equal(t, 1, calls, "prompt should be called exactly once")
	assert.Equal(t, "local-a", readLocalFile(t, filepath.Join(dir, "a.txt")))
	assert.Equal(t, "local-b", readLocalFile(t, filepath.Join(dir, "b.txt")))
	assert.Equal(t, "new-c", readLocalFile(t, filepath.Join(dir, "c.txt")))
}

func TestExtractConflictOverwrite(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local"), 0644))
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote"})
	opts := Options{SourceDir: dir, Out: io.Discard}

	require.NoError(t, extractWithConflicts(archive, opts, fixedPrompt(ChoiceOverwrite)))

	assert.Equal(t, "remote", readLocalFile(t, filepath.Join(dir, "a.txt")))
}

func TestExtractConflictOverwriteAll(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local-a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("local-b"), 0644))

	calls := 0
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote-a", "./b.txt": "remote-b"})
	opts := Options{SourceDir: dir, Out: io.Discard}
	require.NoError(t, extractWithConflicts(archive, opts, func(_ string) (ConflictChoice, error) {
		calls++
		return ChoiceOverwriteAll, nil
	}))

	assert.Equal(t, 1, calls, "prompt should be called exactly once")
	assert.Equal(t, "remote-a", readLocalFile(t, filepath.Join(dir, "a.txt")))
	assert.Equal(t, "remote-b", readLocalFile(t, filepath.Join(dir, "b.txt")))
}

func TestExtractConflictAbort(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local"), 0644))
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote"})
	opts := Options{SourceDir: dir, Out: io.Discard}

	err := extractWithConflicts(archive, opts, fixedPrompt(ChoiceAbort))
	assert.ErrorIs(t, err, ErrAborted)
	assert.Equal(t, "local", readLocalFile(t, filepath.Join(dir, "a.txt")), "a.txt should be unchanged after abort")
}

// --- extractEntry path traversal ---

func TestExtractRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := makeTarGz(t, map[string]string{
		"../../evil.txt": "pwned",
	})
	opts := Options{SourceDir: dir, Out: io.Discard}

	err := extractWithConflicts(archive, opts, noPrompt(t))
	require.Error(t, err)
	var traversalErr *PathTraversalError
	require.ErrorAs(t, err, &traversalErr)
	assert.Contains(t, traversalErr.Entry, "..")
}

// --- applyPattern ---

func TestApplyPattern(t *testing.T) {
	tests := []struct {
		name     string
		excluded bool
		relPath  string
		pattern  string
		want     bool
	}{
		{"blank pattern is no-op", false, "foo.go", "", false},
		{"comment is no-op", false, "foo.go", "# comment", false},
		{"match sets excluded", false, ".git/HEAD", ".git", true},
		{"no match leaves state", true, "main.go", ".git", true},
		{"negated match clears excluded", true, "main.go", "!main.go", false},
		{"negated no-match leaves state", true, "other.go", "!main.go", true},
		{"whitespace trimmed", false, ".git/HEAD", "  .git  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, applyPattern(tt.excluded, tt.relPath, tt.pattern))
		})
	}
}

// --- remoteQuote ---

func TestRemoteQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain path", "/tmp/foo", "'/tmp/foo'"},
		{"path with spaces", "/my dir/foo", "'/my dir/foo'"},
		{"path with single quote", "/tmp/it's", "'/tmp/it'\\''s'"},
		{"tilde alone", "~", `"$HOME"`},
		{"tilde slash prefix", "~/kubernetes", `"$HOME/"'kubernetes'`},
		{"tilde slash nested", "~/kubernetes/foo/bar.yml", `"$HOME/"'kubernetes/foo/bar.yml'`},
		{"tilde slash with spaces", "~/my dir/foo", `"$HOME/"'my dir/foo'`},
		{"tilde slash with single quote", "~/it's", `"$HOME/"'it'\''s'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, remoteQuote(tt.input))
		})
	}
}

// --- buildLSCommand ---

func TestBuildLSCommand(t *testing.T) {
	tests := []struct {
		name       string
		remotePath string
		lsOpts     LSOptions
		want       string
	}{
		{"no flags", "/tmp", LSOptions{}, "ls '/tmp'"},
		{"long", "/tmp", LSOptions{Long: true}, "ls -l '/tmp'"},
		{"all flags", "/tmp", LSOptions{Long: true, All: true, Human: true, Recursive: true}, "ls -lahR '/tmp'"},
		{"path with spaces", "/my dir", LSOptions{}, "ls '/my dir'"},
		{"path with single quote", "/tmp/it's", LSOptions{}, "ls '/tmp/it'\\''s'"},
		{"tilde path", "~/logs", LSOptions{}, `ls "$HOME/"'logs'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildLSCommand(tt.remotePath, tt.lsOpts))
		})
	}
}

// --- writeFile atomicity ---

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "out.txt")

	require.NoError(t, writeFile(absPath, "out.txt", strings.NewReader("hello"), io.Discard))

	assert.Equal(t, "hello", readLocalFile(t, absPath))
}

func TestWriteFileOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "out.txt")
	require.NoError(t, os.WriteFile(absPath, []byte("old"), 0644))

	require.NoError(t, writeFile(absPath, "out.txt", strings.NewReader("new"), io.Discard))

	assert.Equal(t, "new", readLocalFile(t, absPath))
}

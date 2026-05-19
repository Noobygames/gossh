package transfer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// makeTarGz builds an in-memory gzip-compressed tar archive.
func makeTarGz(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		must(t, tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     name,
			Mode:     0644,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		must(t, err)
	}
	must(t, tw.Close())
	must(t, gz.Close())
	return &buf
}

func readLocalFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	return string(data)
}

func noPrompt(t *testing.T) ConflictPromptFn {
	return func(relPath string) (ConflictChoice, error) {
		t.Errorf("unexpected conflict prompt for %q", relPath)
		return 0, nil
	}
}

func fixedPrompt(c ConflictChoice) ConflictPromptFn {
	return func(_ string) (ConflictChoice, error) { return c, nil }
}

// --- isExcluded ---

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		path     string
		excludes []string
		want     bool
	}{
		{".git/config", []string{".git"}, true},
		{"src/.git/config", []string{".git"}, true},
		{"main.go", []string{".git"}, false},
		{"secret.yml", []string{"secret.yml"}, true},
		{"nested/secret.yml", []string{"secret.yml"}, true},
		{"nosecret.yml", []string{"secret.yml"}, false},
		{"a/b/c", []string{"b"}, true},
		{"a/b/c", []string{}, false},
	}
	for _, tt := range tests {
		if got := isExcluded(tt.path, tt.excludes); got != tt.want {
			t.Errorf("isExcluded(%q, %v) = %v, want %v", tt.path, tt.excludes, got, tt.want)
		}
	}
}

// --- ListFiles ---

func TestListFiles(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	must(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644))
	must(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
	must(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("c"), 0644))

	var buf bytes.Buffer
	must(t, ListFiles(&buf, dir, []string{".git"}))
	out := buf.String()

	if !strings.Contains(out, "a.txt") {
		t.Error("expected a.txt in output")
	}
	if !strings.Contains(out, "b.txt") {
		t.Error("expected b.txt in output")
	}
	if !strings.Contains(out, "SKIP") || !strings.Contains(out, ".git") {
		t.Error("expected .git SKIP line")
	}
	if strings.Contains(out, "config") {
		t.Error(".git/config should be hidden by SkipDir")
	}
}

// --- ArchiveAndSend ---

func TestArchiveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"hello.txt": "hello", "sub/world.txt": "world"}
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		must(t, os.MkdirAll(filepath.Dir(path), 0755))
		must(t, os.WriteFile(path, []byte(content), 0644))
	}
	must(t, os.WriteFile(filepath.Join(dir, "secret.yml"), []byte("secret"), 0644))

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		errCh <- ArchiveAndSend(pw, io.Discard, dir, []string{"secret.yml"})
	}()

	gr, err := gzip.NewReader(pr)
	must(t, err)
	tr := tar.NewReader(gr)

	got := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		must(t, err)
		data, err := io.ReadAll(tr)
		must(t, err)
		got[hdr.Name] = string(data)
	}
	must(t, <-errCh)

	for rel, want := range files {
		if got[rel] != want {
			t.Errorf("file %q: got %q, want %q", rel, got[rel], want)
		}
	}
	if _, ok := got["secret.yml"]; ok {
		t.Error("secret.yml should be excluded")
	}
}

// --- Push dry-run ---

func TestPushDryRun(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("deploy"), 0644))
	must(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
	must(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0644))

	var buf bytes.Buffer
	opts := Options{
		SourceDir: dir,
		Server:    "user@host",
		RemoteDir: "~/k8s",
		DryRun:    true,
		Excludes:  []string{".git"},
		Out:       &buf,
	}
	must(t, Push(opts))

	out := buf.String()
	if !strings.Contains(out, "deploy.yaml") {
		t.Error("expected deploy.yaml in dry-run output")
	}
	if strings.Contains(out, "HEAD") {
		t.Error(".git/HEAD should be excluded")
	}
}

// --- extractWithConflicts ---

func TestExtractNoConflicts(t *testing.T) {
	dir := t.TempDir()
	archive := makeTarGz(t, map[string]string{"./a.txt": "hello", "./sub/b.txt": "world"})
	opts := Options{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, opts, noPrompt(t)))

	if readLocalFile(t, filepath.Join(dir, "a.txt")) != "hello" {
		t.Error("a.txt: wrong content")
	}
	if readLocalFile(t, filepath.Join(dir, "sub", "b.txt")) != "world" {
		t.Error("sub/b.txt: wrong content")
	}
}

func TestExtractConflictSkip(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local"), 0644))
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote", "./b.txt": "new"})
	opts := Options{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, opts, fixedPrompt(ChoiceSkip)))

	if readLocalFile(t, filepath.Join(dir, "a.txt")) != "local" {
		t.Error("a.txt should be unchanged")
	}
	if readLocalFile(t, filepath.Join(dir, "b.txt")) != "new" {
		t.Error("b.txt should be written")
	}
}

func TestExtractConflictSkipAll(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local-a"), 0644))
	must(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("local-b"), 0644))

	calls := 0
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote-a", "./b.txt": "remote-b", "./c.txt": "new-c"})
	opts := Options{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, opts, func(_ string) (ConflictChoice, error) {
		calls++
		return ChoiceSkipAll, nil
	}))

	if calls != 1 {
		t.Errorf("prompt called %d times, want 1", calls)
	}
	if readLocalFile(t, filepath.Join(dir, "a.txt")) != "local-a" {
		t.Error("a.txt should be unchanged")
	}
	if readLocalFile(t, filepath.Join(dir, "b.txt")) != "local-b" {
		t.Error("b.txt should be unchanged")
	}
	if readLocalFile(t, filepath.Join(dir, "c.txt")) != "new-c" {
		t.Error("c.txt should be written")
	}
}

func TestExtractConflictOverwrite(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local"), 0644))
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote"})
	opts := Options{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, opts, fixedPrompt(ChoiceOverwrite)))

	if readLocalFile(t, filepath.Join(dir, "a.txt")) != "remote" {
		t.Error("a.txt should be overwritten")
	}
}

func TestExtractConflictOverwriteAll(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local-a"), 0644))
	must(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("local-b"), 0644))

	calls := 0
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote-a", "./b.txt": "remote-b"})
	opts := Options{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, opts, func(_ string) (ConflictChoice, error) {
		calls++
		return ChoiceOverwriteAll, nil
	}))

	if calls != 1 {
		t.Errorf("prompt called %d times, want 1", calls)
	}
	if readLocalFile(t, filepath.Join(dir, "a.txt")) != "remote-a" {
		t.Error("a.txt should be overwritten")
	}
	if readLocalFile(t, filepath.Join(dir, "b.txt")) != "remote-b" {
		t.Error("b.txt should be overwritten")
	}
}

func TestExtractConflictAbort(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local"), 0644))
	archive := makeTarGz(t, map[string]string{"./a.txt": "remote"})
	opts := Options{SourceDir: dir, Out: io.Discard}
	if err := extractWithConflicts(archive, opts, fixedPrompt(ChoiceAbort)); err == nil {
		t.Error("expected error on abort")
	}
	if readLocalFile(t, filepath.Join(dir, "a.txt")) != "local" {
		t.Error("a.txt should be unchanged after abort")
	}
}

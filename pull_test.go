package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// makeTarGz builds an in-memory gzip-compressed tar archive from the given files.
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

func noPrompt(t *testing.T) conflictPromptFn {
	return func(relPath string) (conflictChoice, error) {
		t.Errorf("unexpected conflict prompt for %q", relPath)
		return 0, nil
	}
}

func fixedPrompt(c conflictChoice) conflictPromptFn {
	return func(_ string) (conflictChoice, error) { return c, nil }
}

func TestExtractNoConflicts(t *testing.T) {
	dir := t.TempDir()
	archive := makeTarGz(t, map[string]string{
		"./a.txt":     "hello",
		"./sub/b.txt": "world",
	})
	cfg := SyncConfig{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, cfg, noPrompt(t)))

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

	archive := makeTarGz(t, map[string]string{
		"./a.txt": "remote",
		"./b.txt": "new",
	})
	cfg := SyncConfig{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, cfg, fixedPrompt(choiceSkip)))

	if readLocalFile(t, filepath.Join(dir, "a.txt")) != "local" {
		t.Error("a.txt should be unchanged after skip")
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
	archive := makeTarGz(t, map[string]string{
		"./a.txt": "remote-a",
		"./b.txt": "remote-b",
		"./c.txt": "new-c",
	})
	cfg := SyncConfig{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, cfg, func(_ string) (conflictChoice, error) {
		calls++
		return choiceSkipAll, nil
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
		t.Error("c.txt should be written (no conflict)")
	}
}

func TestExtractConflictOverwrite(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local"), 0644))

	archive := makeTarGz(t, map[string]string{"./a.txt": "remote"})
	cfg := SyncConfig{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, cfg, fixedPrompt(choiceOverwrite)))

	if readLocalFile(t, filepath.Join(dir, "a.txt")) != "remote" {
		t.Error("a.txt should be overwritten")
	}
}

func TestExtractConflictOverwriteAll(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("local-a"), 0644))
	must(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("local-b"), 0644))

	calls := 0
	archive := makeTarGz(t, map[string]string{
		"./a.txt": "remote-a",
		"./b.txt": "remote-b",
	})
	cfg := SyncConfig{SourceDir: dir, Out: io.Discard}
	must(t, extractWithConflicts(archive, cfg, func(_ string) (conflictChoice, error) {
		calls++
		return choiceOverwriteAll, nil
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
	cfg := SyncConfig{SourceDir: dir, Out: io.Discard}
	if err := extractWithConflicts(archive, cfg, fixedPrompt(choiceAbort)); err == nil {
		t.Error("expected error on abort")
	}
	if readLocalFile(t, filepath.Join(dir, "a.txt")) != "local" {
		t.Error("a.txt should be unchanged after abort")
	}
}

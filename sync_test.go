package main

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
		// component match, not substring — "nosecret.yml" must not match "secret.yml"
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

func TestListFiles(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	must(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644))
	must(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
	must(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("c"), 0644))

	var buf bytes.Buffer
	must(t, listFiles(&buf, dir, []string{".git"}))
	out := buf.String()

	if !strings.Contains(out, "a.txt") {
		t.Error("expected a.txt in output")
	}
	if !strings.Contains(out, "b.txt") {
		t.Error("expected b.txt in output")
	}
	if !strings.Contains(out, "SKIP") || !strings.Contains(out, ".git") {
		t.Error("expected .git SKIP line in output")
	}
	if strings.Contains(out, "config") {
		t.Error(".git/config should be hidden by SkipDir, not printed")
	}
}

func TestArchiveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"hello.txt":     "hello",
		"sub/world.txt": "world",
	}
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		must(t, os.MkdirAll(filepath.Dir(path), 0755))
		must(t, os.WriteFile(path, []byte(content), 0644))
	}
	must(t, os.WriteFile(filepath.Join(dir, "secret.yml"), []byte("secret"), 0644))

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		errCh <- archiveAndSend(pw, io.Discard, dir, []string{"secret.yml"})
	}()

	gr, err := gzip.NewReader(pr)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gr)

	got := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		got[hdr.Name] = string(data)
	}

	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	for rel, want := range files {
		if got[rel] != want {
			t.Errorf("file %q: got %q, want %q", rel, got[rel], want)
		}
	}
	if _, ok := got["secret.yml"]; ok {
		t.Error("secret.yml should have been excluded from the archive")
	}
}

func TestRunDryRun(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte("deploy"), 0644))
	must(t, os.MkdirAll(filepath.Join(dir, ".git"), 0755))
	must(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0644))

	var buf bytes.Buffer
	cfg := SyncConfig{
		SourceDir: dir,
		Server:    "user@host",
		RemoteDir: "~/k8s",
		DryRun:    true,
		Excludes:  []string{".git"},
		Out:       &buf,
	}
	must(t, run(cfg))

	out := buf.String()
	if !strings.Contains(out, "deploy.yaml") {
		t.Error("expected deploy.yaml in dry-run output")
	}
	if strings.Contains(out, "HEAD") {
		t.Error(".git/HEAD should be excluded in dry-run")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

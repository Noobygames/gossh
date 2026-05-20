package transfer

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/noobygames/gossh/pkg/config"
)

type fileVisitor func(absPath, relPath string, info fs.FileInfo) error

func walkDir(sourceDir string, fn func(absPath string, d fs.DirEntry, relPath string) error) error {
	abs, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	return filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		return fn(path, d, filepath.ToSlash(rel))
	})
}

func walkFiltered(sourceDir string, excludes []string, visit fileVisitor) error {
	return walkDir(sourceDir, func(absPath string, d fs.DirEntry, relPath string) error {
		if isExcluded(relPath, excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return visit(absPath, relPath, info)
	})
}

// ArchiveAndSend packs sourceDir (excluding excludes) into a gzip-compressed
// tar stream written to w. w is always closed, on both success and failure.
func ArchiveAndSend(w io.WriteCloser, out io.Writer, sourceDir string, excludes []string) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	if err := writeTar(tw, out, sourceDir, excludes); err != nil {
		_ = w.Close()
		return fmt.Errorf("tar: %w", err)
	}
	if err := tw.Close(); err != nil {
		_ = w.Close()
		return fmt.Errorf("tar close: %w", err)
	}
	if err := gz.Close(); err != nil {
		_ = w.Close()
		return fmt.Errorf("gzip close: %w", err)
	}
	return w.Close()
}

func writeTar(tw *tar.Writer, out io.Writer, sourceDir string, excludes []string) error {
	return walkFiltered(sourceDir, excludes, func(absPath, relPath string, info fs.FileInfo) error {
		return addFile(tw, out, absPath, relPath, info)
	})
}

func addFile(tw *tar.Writer, out io.Writer, absPath, relPath string, info fs.FileInfo) error {
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = relPath
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = io.Copy(tw, f); err != nil {
		return err
	}
	fmt.Fprintf(out, "  add   %s\n", relPath)
	return nil
}

// ListFiles prints the files that would be synced from sourceDir, used by dry-run.
func ListFiles(out io.Writer, sourceDir string, excludes []string) error {
	return walkDir(sourceDir, func(_ string, d fs.DirEntry, relPath string) error {
		if isExcluded(relPath, excludes) {
			fmt.Fprintf(out, "  SKIP  %s\n", relPath)
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			fmt.Fprintf(out, "  sync  %s\n", relPath)
		}
		return nil
	})
}

func isExcluded(relPath string, excludes []string) bool {
	excluded := false
	for _, p := range excludes {
		excluded = applyPattern(excluded, relPath, p)
	}
	return excluded
}

// applyPattern evaluates one gitignore-style pattern against relPath and
// returns the updated excluded state (last-match-wins semantics).
func applyPattern(excluded bool, relPath, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || strings.HasPrefix(pattern, "#") {
		return excluded
	}
	negated := strings.HasPrefix(pattern, "!")
	if negated {
		pattern = pattern[1:]
	}
	if config.MatchPattern(relPath, pattern) {
		return !negated
	}
	return excluded
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

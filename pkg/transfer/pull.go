package transfer

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/noobygames/gossh/pkg/sshconn"
)

// ConflictChoice represents the user's resolution for a file conflict during pull.
type ConflictChoice int

const (
	ChoiceSkip ConflictChoice = iota
	ChoiceSkipAll
	ChoiceOverwrite
	ChoiceOverwriteAll
	ChoiceAbort
)

// ConflictPromptFn is called when a pulled file would overwrite an existing local file.
type ConflictPromptFn func(relPath string) (ConflictChoice, error)

// TerminalConflictPrompt reads a conflict resolution choice from the terminal.
func TerminalConflictPrompt(relPath string) (ConflictChoice, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "  conflict  %s\n    [s]kip  [S]kip all  [o]verwrite  [O]verwrite all  [a]bort: ", relPath)
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, fmt.Errorf("reading choice: %w", err)
		}
		switch strings.TrimSpace(line) {
		case "s":
			return ChoiceSkip, nil
		case "S":
			return ChoiceSkipAll, nil
		case "o":
			return ChoiceOverwrite, nil
		case "O":
			return ChoiceOverwriteAll, nil
		case "a":
			return ChoiceAbort, nil
		}
	}
}

// conflictState tracks skip-all / overwrite-all decisions across a pull session.
type conflictState struct {
	skipAll      bool
	overwriteAll bool
}

// resolve decides whether to write a conflicting file.
// Returns (true, nil) to write, (false, nil) to skip, (false, err) to abort.
func (s *conflictState) resolve(relPath string, prompt ConflictPromptFn, out io.Writer) (write bool, err error) {
	if s.skipAll {
		fmt.Fprintf(out, "  skip  %s\n", relPath)
		return false, nil
	}
	if s.overwriteAll {
		return true, nil
	}
	choice, err := prompt(relPath)
	if err != nil {
		return false, err
	}
	switch choice {
	case ChoiceSkip:
		fmt.Fprintf(out, "  skip  %s\n", relPath)
		return false, nil
	case ChoiceSkipAll:
		s.skipAll = true
		fmt.Fprintf(out, "  skip  %s\n", relPath)
		return false, nil
	case ChoiceOverwrite:
		return true, nil
	case ChoiceOverwriteAll:
		s.overwriteAll = true
		return true, nil
	case ChoiceAbort:
		return false, ErrAborted
	default:
		return false, fmt.Errorf("unknown conflict choice %d", choice)
	}
}

// Pull downloads opts.RemoteDir to opts.SourceDir, prompting on conflicts.
func Pull(ctx context.Context, opts Options, prompt ConflictPromptFn) error {
	fmt.Fprintf(opts.Out, "Connecting to %s...\n", opts.Server)
	client, err := sshconn.Connect(ctx, opts.Server, opts.Identity, opts.prompt())
	if err != nil {
		return err
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	defer sess.Close()
	sess.Stderr = os.Stderr

	r, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := sess.Start(fmt.Sprintf("tar -czf - -C %s .", shellQuote(opts.RemoteDir))); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(opts.Out, "Pulling %s:%s → %s\n", opts.Server, opts.RemoteDir, opts.SourceDir)
	if err := extractWithConflicts(r, opts, prompt); err != nil {
		return err
	}
	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	fmt.Fprintln(opts.Out, "\nDone.")
	return nil
}

func extractWithConflicts(r io.Reader, opts Options, prompt ConflictPromptFn) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	tr := tar.NewReader(gr)
	var state conflictState

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if err := extractEntry(hdr, tr, opts.SourceDir, &state, prompt, opts.Out); err != nil {
			return err
		}
	}
	return nil
}

func extractEntry(hdr *tar.Header, r io.Reader, destDir string, state *conflictState, prompt ConflictPromptFn, out io.Writer) error {
	relPath := filepath.FromSlash(strings.TrimPrefix(hdr.Name, "./"))
	if relPath == "" {
		return nil
	}
	absPath := filepath.Join(destDir, relPath)

	// Reject path traversal: a malicious server could supply entries like
	// "../../etc/passwd". filepath.Join cleans ".." but does not prevent escape.
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(absPath)+string(os.PathSeparator), cleanDest) {
		return &PathTraversalError{Entry: hdr.Name}
	}

	if _, err := os.Lstat(absPath); err == nil {
		write, err := state.resolve(relPath, prompt, out)
		if err != nil {
			return err
		}
		if !write {
			return nil
		}
	}
	return writeFile(absPath, relPath, r, out)
}

// writeFile writes r atomically to absPath via a temp file + rename.
func writeFile(absPath, relPath string, r io.Reader, out io.Writer) error {
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".gossh-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), absPath); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	fmt.Fprintf(out, "  pull  %s\n", relPath)
	return nil
}

// PullFile downloads one specific remote file to localPath, prompting on conflict.
func PullFile(ctx context.Context, opts Options, remotePath, localPath string, prompt ConflictPromptFn) error {
	if _, err := os.Lstat(localPath); err == nil {
		choice, err := prompt(localPath)
		if err != nil {
			return err
		}
		switch choice {
		case ChoiceSkip, ChoiceSkipAll:
			fmt.Fprintf(opts.Out, "  skip  %s\n", localPath)
			return nil
		case ChoiceAbort:
			return ErrAborted
		}
	}

	fmt.Fprintf(opts.Out, "Connecting to %s...\n", opts.Server)
	client, err := sshconn.Connect(ctx, opts.Server, opts.Identity, opts.prompt())
	if err != nil {
		return err
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	defer sess.Close()
	sess.Stderr = os.Stderr

	r, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := sess.Start(fmt.Sprintf("cat %s", shellQuote(remotePath))); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(opts.Out, "Pulling %s:%s → %s\n", opts.Server, remotePath, localPath)
	if err := writeFile(localPath, localPath, r, opts.Out); err != nil {
		return err
	}
	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	fmt.Fprintln(opts.Out, "Done.")
	return nil
}

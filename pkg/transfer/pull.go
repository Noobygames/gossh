package transfer

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
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

// Pull downloads opts.RemoteDir to opts.SourceDir, prompting on conflicts.
func Pull(opts Options, prompt ConflictPromptFn) error {
	fmt.Fprintf(opts.Out, "Connecting to %s...\n", opts.Server)
	client, err := sshconn.Connect(opts.Server, opts.Identity, sshconn.TerminalPrompt)
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

	if err := sess.Start(fmt.Sprintf("tar -czf - -C %s .", opts.RemoteDir)); err != nil {
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

	skipAll := false
	overwriteAll := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		relPath := filepath.FromSlash(strings.TrimPrefix(hdr.Name, "./"))
		if relPath == "" {
			continue
		}
		absPath := filepath.Join(opts.SourceDir, relPath)

		if _, err := os.Lstat(absPath); err == nil {
			switch {
			case skipAll:
				fmt.Fprintf(opts.Out, "  skip  %s\n", relPath)
				continue
			case overwriteAll:
				// fall through
			default:
				choice, err := prompt(relPath)
				if err != nil {
					return err
				}
				switch choice {
				case ChoiceSkip:
					fmt.Fprintf(opts.Out, "  skip  %s\n", relPath)
					continue
				case ChoiceSkipAll:
					skipAll = true
					fmt.Fprintf(opts.Out, "  skip  %s\n", relPath)
					continue
				case ChoiceOverwrite:
					// fall through
				case ChoiceOverwriteAll:
					overwriteAll = true
				case ChoiceAbort:
					return fmt.Errorf("aborted by user")
				}
			}
		}
		if err := writeFile(absPath, relPath, tr, opts.Out); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(absPath, relPath string, r io.Reader, out io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(absPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	fmt.Fprintf(out, "  pull  %s\n", relPath)
	return nil
}

// PullFile downloads one specific remote file to localPath, prompting on conflict.
func PullFile(opts Options, remotePath, localPath string, prompt ConflictPromptFn) error {
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
			return fmt.Errorf("aborted by user")
		}
	}

	fmt.Fprintf(opts.Out, "Connecting to %s...\n", opts.Server)
	client, err := sshconn.Connect(opts.Server, opts.Identity, sshconn.TerminalPrompt)
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

	if err := sess.Start(fmt.Sprintf("cat %s", remotePath)); err != nil {
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

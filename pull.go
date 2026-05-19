package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type conflictChoice int

const (
	choiceSkip conflictChoice = iota
	choiceSkipAll
	choiceOverwrite
	choiceOverwriteAll
	choiceAbort
)

type conflictPromptFn func(relPath string) (conflictChoice, error)

func terminalConflictPrompt(relPath string) (conflictChoice, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "  conflict  %s\n    [s]kip  [S]kip all  [o]verwrite  [O]verwrite all  [a]bort: ", relPath)
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, fmt.Errorf("reading choice: %w", err)
		}
		switch strings.TrimSpace(line) {
		case "s":
			return choiceSkip, nil
		case "S":
			return choiceSkipAll, nil
		case "o":
			return choiceOverwrite, nil
		case "O":
			return choiceOverwriteAll, nil
		case "a":
			return choiceAbort, nil
		}
	}
}

func pullFiles(cfg SyncConfig, prompt conflictPromptFn) error {
	fmt.Fprintf(cfg.Out, "Connecting to %s...\n", cfg.Server)
	client, err := connect(cfg, terminalPrompt)
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

	if err := sess.Start(fmt.Sprintf("tar -czf - -C %s .", cfg.RemoteDir)); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(cfg.Out, "Pulling %s:%s → %s\n", cfg.Server, cfg.RemoteDir, cfg.SourceDir)

	if err := extractWithConflicts(r, cfg, prompt); err != nil {
		return err
	}

	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote: %w", err)
	}

	fmt.Fprintln(cfg.Out, "\nDone.")
	return nil
}

func extractWithConflicts(r io.Reader, cfg SyncConfig, prompt conflictPromptFn) error {
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
		// Only handle regular files; skip dirs, symlinks, etc.
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		// tar archives from "tar -czf - -C dir ." use "./" prefixed paths.
		relPath := filepath.FromSlash(strings.TrimPrefix(hdr.Name, "./"))
		if relPath == "" {
			continue
		}

		absPath := filepath.Join(cfg.SourceDir, relPath)

		if _, err := os.Lstat(absPath); err == nil {
			switch {
			case skipAll:
				fmt.Fprintf(cfg.Out, "  skip  %s\n", relPath)
				continue
			case overwriteAll:
				// fall through to write
			default:
				choice, err := prompt(relPath)
				if err != nil {
					return err
				}
				switch choice {
				case choiceSkip:
					fmt.Fprintf(cfg.Out, "  skip  %s\n", relPath)
					continue
				case choiceSkipAll:
					skipAll = true
					fmt.Fprintf(cfg.Out, "  skip  %s\n", relPath)
					continue
				case choiceOverwrite:
					// fall through
				case choiceOverwriteAll:
					overwriteAll = true
					// fall through
				case choiceAbort:
					return fmt.Errorf("aborted by user")
				}
			}
		}

		if err := writeExtractedFile(absPath, relPath, tr, cfg.Out); err != nil {
			return err
		}
	}

	return nil
}

func writeExtractedFile(absPath, relPath string, r io.Reader, out io.Writer) error {
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

// pullSingleFile downloads one specific remote file to localPath.
func pullSingleFile(cfg SyncConfig, remotePath, localPath string, prompt conflictPromptFn) error {
	if _, err := os.Lstat(localPath); err == nil {
		choice, err := prompt(localPath)
		if err != nil {
			return err
		}
		switch choice {
		case choiceSkip, choiceSkipAll:
			fmt.Fprintf(cfg.Out, "  skip  %s\n", localPath)
			return nil
		case choiceAbort:
			return fmt.Errorf("aborted by user")
		}
	}

	fmt.Fprintf(cfg.Out, "Connecting to %s...\n", cfg.Server)
	client, err := connect(cfg, terminalPrompt)
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

	fmt.Fprintf(cfg.Out, "Pulling %s:%s → %s\n", cfg.Server, remotePath, localPath)
	if err := writeExtractedFile(localPath, localPath, r, cfg.Out); err != nil {
		return err
	}

	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	fmt.Fprintln(cfg.Out, "Done.")
	return nil
}

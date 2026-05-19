package main

import (
	"fmt"
	"io"
	"os"
	"path"
)

func run(cfg SyncConfig) error {
	if cfg.DryRun {
		fmt.Fprintf(cfg.Out, "Dry run — would sync: %s → %s:%s\n\n", cfg.SourceDir, cfg.Server, cfg.RemoteDir)
		return listFiles(cfg.Out, cfg.SourceDir, cfg.Excludes)
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
	sess.Stdout = cfg.Out
	sess.Stderr = os.Stderr

	pipe, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	cmd := fmt.Sprintf("mkdir -p %s && tar -xzf - -C %s", cfg.RemoteDir, cfg.RemoteDir)
	if err := sess.Start(cmd); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(cfg.Out, "Syncing %s → %s:%s\n", cfg.SourceDir, cfg.Server, cfg.RemoteDir)

	if err := archiveAndSend(pipe, cfg.Out, cfg.SourceDir, cfg.Excludes); err != nil {
		return err
	}

	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote: %w", err)
	}

	fmt.Fprintln(cfg.Out, "\nDone.")
	return nil
}

// pushFile uploads a single local file to an exact remote path.
func pushFile(cfg SyncConfig, localPath, remotePath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
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

	pipe, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	remoteParent := path.Dir(remotePath)
	cmd := fmt.Sprintf("mkdir -p %s && cat > %s", remoteParent, remotePath)
	if err := sess.Start(cmd); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(cfg.Out, "Pushing %s (%d B) → %s:%s\n", localPath, info.Size(), cfg.Server, remotePath)
	if _, err := io.Copy(pipe, f); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	if err := pipe.Close(); err != nil {
		return fmt.Errorf("pipe close: %w", err)
	}

	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	fmt.Fprintln(cfg.Out, "Done.")
	return nil
}

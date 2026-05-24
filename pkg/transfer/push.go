package transfer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/noobygames/gossh/pkg/sshconn"
)

// Push uploads opts.SourceDir to opts.RemoteDir on the remote host.
func Push(ctx context.Context, opts Options) error {
	if opts.DryRun {
		fmt.Fprintf(opts.Out, "Dry run — would sync: %s → %s:%s\n\n", opts.SourceDir, opts.Server, opts.RemoteDir)
		return ListFiles(opts.Out, opts.SourceDir, opts.Excludes)
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
	sess.Stdout = opts.Out
	sess.Stderr = os.Stderr

	pipe, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	remoteDir := remoteQuote(opts.RemoteDir)
	if err := sess.Start(fmt.Sprintf("mkdir -p %s && tar -xzf - -C %s", remoteDir, remoteDir)); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(opts.Out, "Syncing %s → %s:%s\n", opts.SourceDir, opts.Server, opts.RemoteDir)
	if err := ArchiveAndSend(pipe, opts.Out, opts.SourceDir, opts.Excludes); err != nil {
		return err
	}
	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	fmt.Fprintln(opts.Out, "\nDone.")
	return nil
}

// PushFile uploads a single local file to an exact remote path.
func PushFile(ctx context.Context, opts Options, localPath, remotePath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
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

	pipe, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	remoteDir := remoteQuote(path.Dir(remotePath))
	remoteFile := remoteQuote(remotePath)
	if err := sess.Start(fmt.Sprintf("mkdir -p %s && cat > %s", remoteDir, remoteFile)); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(opts.Out, "Pushing %s (%d B) → %s:%s\n", localPath, info.Size(), opts.Server, remotePath)
	if _, err := io.Copy(pipe, f); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	if err := pipe.Close(); err != nil {
		return fmt.Errorf("pipe close: %w", err)
	}
	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	fmt.Fprintln(opts.Out, "Done.")
	return nil
}

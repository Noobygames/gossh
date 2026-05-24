package transfer

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/noobygames/gossh/pkg/sshconn"
)

// LSOptions controls display behaviour of the ls command.
type LSOptions struct {
	Long      bool
	All       bool
	Human     bool
	Recursive bool
}

// LS lists files at remotePath on the remote host.
func LS(ctx context.Context, opts Options, remotePath string, lsOpts LSOptions) error {
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

	return sess.Run(buildLSCommand(remotePath, lsOpts))
}

func buildLSCommand(remotePath string, lsOpts LSOptions) string {
	var sb strings.Builder
	sb.WriteString("ls")
	if lsOpts.Long || lsOpts.All || lsOpts.Human || lsOpts.Recursive {
		sb.WriteString(" -")
		if lsOpts.Long {
			sb.WriteByte('l')
		}
		if lsOpts.All {
			sb.WriteByte('a')
		}
		if lsOpts.Human {
			sb.WriteByte('h')
		}
		if lsOpts.Recursive {
			sb.WriteByte('R')
		}
	}
	sb.WriteByte(' ')
	sb.WriteString(remoteQuote(remotePath))
	return sb.String()
}

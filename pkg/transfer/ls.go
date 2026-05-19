package transfer

import (
	"fmt"
	"os"
	"strings"

	"github.com/noobygames/gossh/pkg/sshconn"
)

// LS lists files at remotePath on the remote host.
func LS(opts Options, remotePath string, long, all, human, recursive bool) error {
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
	sess.Stdout = opts.Out
	sess.Stderr = os.Stderr

	var sb strings.Builder
	sb.WriteString("ls")
	if long || all || human || recursive {
		sb.WriteString(" -")
		if long {
			sb.WriteByte('l')
		}
		if all {
			sb.WriteByte('a')
		}
		if human {
			sb.WriteByte('h')
		}
		if recursive {
			sb.WriteByte('R')
		}
	}
	sb.WriteByte(' ')
	sb.WriteString(remotePath)

	return sess.Run(sb.String())
}

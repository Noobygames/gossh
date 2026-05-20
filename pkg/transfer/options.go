package transfer

import (
	"io"

	"github.com/noobygames/gossh/pkg/sshconn"
)

// Options holds all parameters for a transfer operation.
type Options struct {
	SourceDir        string
	Server           string
	RemoteDir        string
	Identity         string
	Excludes         []string
	DryRun           bool
	Out              io.Writer
	PassphrasePrompt sshconn.PassphrasePrompt
}

func (o Options) prompt() sshconn.PassphrasePrompt {
	if o.PassphrasePrompt != nil {
		return o.PassphrasePrompt
	}
	return sshconn.TerminalPrompt
}

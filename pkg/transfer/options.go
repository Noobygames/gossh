package transfer

import "io"

// Options holds all parameters for a transfer operation.
type Options struct {
	SourceDir string
	Server    string
	RemoteDir string
	Identity  string
	Excludes  []string
	DryRun    bool
	Out       io.Writer
}

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/noobygames/gossh/pkg/config"
	"github.com/noobygames/gossh/pkg/transfer"
)

func cmdPull(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	fs.Parse(args) //nolint:errcheck // ExitOnError

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runPull(ctx, *server, *remoteDir, *identity, fs.Args(), cfg, os.Stdout)
}

func runPull(ctx context.Context, server, remoteDir, identity string, positional []string, cfg config.Config, out io.Writer) error {
	opts := transfer.Options{
		Server:    config.FirstNonEmpty(server, cfg.Server),
		RemoteDir: config.FirstNonEmpty(remoteDir, cfg.RemoteDir, "~/kubernetes"),
		Identity:  config.FirstNonEmpty(identity),
		Out:       out,
	}

	switch len(positional) {
	case 0, 1:
		opts.SourceDir = "."
		if len(positional) == 1 {
			opts.SourceDir = positional[0]
		}
		return transfer.Pull(ctx, opts, transfer.TerminalConflictPrompt)
	case 2:
		return transfer.PullFile(ctx, opts, positional[0], positional[1], transfer.TerminalConflictPrompt)
	default:
		return fmt.Errorf("too many arguments — run 'gossh help pull'")
	}
}

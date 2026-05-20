package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/noobygames/gossh/pkg/config"
	"github.com/noobygames/gossh/pkg/transfer"
)

func cmdPush(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "List files without transferring")
	var extra stringSlice
	fs.Var(&extra, "exclude", "Additional exclude pattern (repeatable, gitignore-style)")
	fs.Parse(args) //nolint:errcheck // ExitOnError

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runPush(ctx, *server, *remoteDir, *identity, *dryRun, []string(extra), fs.Args(), cfg, os.Stdout)
}

func runPush(ctx context.Context, server, remoteDir, identity string, dryRun bool, extraExcludes, positional []string, cfg config.Config, out io.Writer) error {
	opts := transfer.Options{
		Server:    config.FirstNonEmpty(server, cfg.Server),
		RemoteDir: config.FirstNonEmpty(remoteDir, cfg.RemoteDir, "~/kubernetes"),
		DryRun:    dryRun,
		Identity:  config.FirstNonEmpty(identity),
		Excludes:  slices.Concat(defaultExcludes, cfg.Excludes, extraExcludes),
		Out:       out,
	}

	switch len(positional) {
	case 0, 1:
		opts.SourceDir = "."
		if len(positional) == 1 {
			opts.SourceDir = positional[0]
		}
		return transfer.Push(ctx, opts)
	case 2:
		localPath, remotePath := positional[0], positional[1]
		info, err := os.Stat(localPath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			opts.SourceDir = localPath
			opts.RemoteDir = remotePath
			return transfer.Push(ctx, opts)
		}
		return transfer.PushFile(ctx, opts, localPath, remotePath)
	default:
		return fmt.Errorf("too many arguments — run 'gossh help push'")
	}
}

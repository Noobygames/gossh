package main

import (
	"context"
	"flag"
	"io"
	"os"

	"github.com/noobygames/gossh/pkg/config"
	"github.com/noobygames/gossh/pkg/transfer"
)

func cmdLS(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	long := fs.Bool("l", false, "Long listing format")
	all := fs.Bool("a", false, "Include hidden entries")
	human := fs.Bool("h", false, "Human-readable sizes (with -l)")
	recursive := fs.Bool("R", false, "Recursive")
	fs.Parse(args) //nolint:errcheck // ExitOnError

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runLS(ctx, *server, *remoteDir, *identity, fs.Arg(0), transfer.LSOptions{
		Long:      *long,
		All:       *all,
		Human:     *human,
		Recursive: *recursive,
	}, cfg, os.Stdout)
}

func runLS(ctx context.Context, server, remoteDir, identity, pathArg string, lsOpts transfer.LSOptions, cfg config.Config, out io.Writer) error {
	return transfer.LS(ctx, transfer.Options{
		Server:   config.FirstNonEmpty(server, cfg.Server),
		Identity: config.FirstNonEmpty(identity),
		Out:      out,
	}, resolveRemotePath(pathArg, remoteDir, cfg), lsOpts)
}

// resolveRemotePath returns the effective remote path for ls and pull operations,
// applying priority: explicit path arg > flag > config > default.
func resolveRemotePath(pathArg, remoteDir string, cfg config.Config) string {
	return config.FirstNonEmpty(pathArg, remoteDir, cfg.RemoteDir, "~/kubernetes")
}

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func cmdLS(args []string) error {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	long := fs.Bool("l", false, "Long listing format")
	all := fs.Bool("a", false, "Include hidden entries (starting with .)")
	human := fs.Bool("h", false, "Human-readable file sizes (with -l)")
	recursive := fs.Bool("R", false, "List subdirectories recursively")
	fs.Parse(args) //nolint:errcheck // ExitOnError

	fileCfg, err := loadConfig()
	if err != nil {
		return err
	}

	remotePath := firstNonEmpty(*remoteDir, fileCfg.RemoteDir, "~/kubernetes")
	if fs.NArg() > 0 {
		remotePath = fs.Arg(0)
	}

	return runLS(SyncConfig{
		Server:   firstNonEmpty(*server, fileCfg.Server),
		Identity: firstNonEmpty(*identity),
		Out:      os.Stdout,
	}, remotePath, *long, *all, *human, *recursive)
}

func runLS(cfg SyncConfig, remotePath string, long, all, human, recursive bool) error {
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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
)

var defaultExcludes = []string{
	".git",
	"sealed-secrets-master-key-backup.yaml",
	"gossh",
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		cmdHelp(nil)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch os.Args[1] {
	case "push":
		err = cmdPush(ctx, os.Args[2:])
	case "pull":
		err = cmdPull(ctx, os.Args[2:])
	case "ls":
		err = cmdLS(ctx, os.Args[2:])
	case "kubectl":
		err = cmdTool(ctx, "kubectl", os.Args[2:])
	case "helm":
		err = cmdTool(ctx, "helm", os.Args[2:])
	case "completion":
		cmdCompletion(os.Args[2:])
	case "version", "--version", "-version":
		cmdVersion()
	case "help", "-h", "--help", "-help":
		cmdHelp(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		cmdHelp(nil)
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

// normalizeRemotePath converts a remote path that the local shell may have
// expanded (e.g. PowerShell turns ~/foo into C:\Users\name/foo) back into a
// portable form: backslashes become forward slashes, and the local home-dir
// prefix is replaced with ~ so the remote shell can resolve it correctly.
func normalizeRemotePath(p string) string {
	p = filepath.ToSlash(p)
	if home, err := os.UserHomeDir(); err == nil {
		homeSlash := filepath.ToSlash(home)
		if p == homeSlash {
			return "~"
		}
		if strings.HasPrefix(p, homeSlash+"/") {
			return "~/" + p[len(homeSlash)+1:]
		}
	}
	return p
}

func commonFlags(fs *flag.FlagSet) (server, remoteDir, identity *string) {
	server = fs.String("server", "", "SSH target (user@host[:port])")
	remoteDir = fs.String("remote-dir", "", "Remote directory (default: from config or ~/kubernetes)")
	identity = fs.String("identity", "", "SSH private key (default: ~/.ssh/id_ed25519, ~/.ssh/id_rsa, ~/.ssh/id_ecdsa)")
	return
}

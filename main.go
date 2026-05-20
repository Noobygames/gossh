package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
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

func commonFlags(fs *flag.FlagSet) (server, remoteDir, identity *string) {
	server = fs.String("server", "", "SSH target (user@host[:port])")
	remoteDir = fs.String("remote-dir", "", "Remote directory (default: from config or ~/kubernetes)")
	identity = fs.String("identity", "", "SSH private key (default: ~/.ssh/id_ed25519, ~/.ssh/id_rsa, ~/.ssh/id_ecdsa)")
	return
}

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
)

var defaultExcludes = []string{
	".git",
	"sealed-secrets-master-key-backup.yaml",
	"gomvtossh",
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gomvtossh <push|pull> [flags] [dir]")
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "push":
		err = cmdPush(os.Args[2:])
	case "pull":
		err = cmdPull(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q — expected push or pull\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func commonFlags(fs *flag.FlagSet) (server, remoteDir, identity *string) {
	server = fs.String("server", "", "SSH target (user@host[:port])")
	remoteDir = fs.String("remote-dir", "", "Remote destination directory (overrides config; default: ~/kubernetes)")
	identity = fs.String("identity", "", "SSH private key (default: ~/.ssh/id_ed25519, ~/.ssh/id_rsa, ~/.ssh/id_ecdsa)")
	return
}

func cmdPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "List files without transferring")
	var extra stringSlice
	fs.Var(&extra, "exclude", "Additional exclude pattern (repeatable, gitignore-style)")
	fs.Parse(args) //nolint:errcheck // ExitOnError

	fileCfg, err := loadConfig()
	if err != nil {
		return err
	}

	sourceDir := "."
	if fs.NArg() > 0 {
		sourceDir = fs.Arg(0)
	}
	return run(SyncConfig{
		SourceDir: sourceDir,
		Server:    firstNonEmpty(*server, fileCfg.Server),
		RemoteDir: firstNonEmpty(*remoteDir, fileCfg.RemoteDir, "~/kubernetes"),
		DryRun:    *dryRun,
		Identity:  firstNonEmpty(*identity),
		Excludes:  slices.Concat(defaultExcludes, fileCfg.Excludes, []string(extra)),
		Out:       os.Stdout,
	})
}

func cmdPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	fs.Parse(args) //nolint:errcheck // ExitOnError

	fileCfg, err := loadConfig()
	if err != nil {
		return err
	}

	localDir := "."
	if fs.NArg() > 0 {
		localDir = fs.Arg(0)
	}
	return pullFiles(SyncConfig{
		SourceDir: localDir,
		Server:    firstNonEmpty(*server, fileCfg.Server),
		RemoteDir: firstNonEmpty(*remoteDir, fileCfg.RemoteDir, "~/kubernetes"),
		Identity:  firstNonEmpty(*identity),
		Out:       os.Stdout,
	}, terminalConflictPrompt)
}

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

	var err error
	switch os.Args[1] {
	case "push":
		err = cmdPush(os.Args[2:])
	case "pull":
		err = cmdPull(os.Args[2:])
	case "ls":
		err = cmdLS(os.Args[2:])
	case "kubectl":
		err = cmdKubectl(os.Args[2:])
	case "helm":
		err = cmdHelm(os.Args[2:])
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

	cfg := SyncConfig{
		Server:    firstNonEmpty(*server, fileCfg.Server),
		RemoteDir: firstNonEmpty(*remoteDir, fileCfg.RemoteDir, "~/kubernetes"),
		DryRun:    *dryRun,
		Identity:  firstNonEmpty(*identity),
		Excludes:  slices.Concat(defaultExcludes, fileCfg.Excludes, []string(extra)),
		Out:       os.Stdout,
	}

	switch fs.NArg() {
	case 0:
		cfg.SourceDir = "."
		return run(cfg)
	case 1:
		cfg.SourceDir = fs.Arg(0)
		return run(cfg)
	case 2:
		localPath, remotePath := fs.Arg(0), fs.Arg(1)
		info, err := os.Stat(localPath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			cfg.SourceDir = localPath
			cfg.RemoteDir = remotePath
			return run(cfg)
		}
		return pushFile(cfg, localPath, remotePath)
	default:
		return fmt.Errorf("too many arguments — run 'gossh help push'")
	}
}

func cmdPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	fs.Parse(args) //nolint:errcheck // ExitOnError

	fileCfg, err := loadConfig()
	if err != nil {
		return err
	}

	cfg := SyncConfig{
		Server:    firstNonEmpty(*server, fileCfg.Server),
		RemoteDir: firstNonEmpty(*remoteDir, fileCfg.RemoteDir, "~/kubernetes"),
		Identity:  firstNonEmpty(*identity),
		Out:       os.Stdout,
	}

	switch fs.NArg() {
	case 0:
		cfg.SourceDir = "."
		return pullFiles(cfg, terminalConflictPrompt)
	case 1:
		cfg.SourceDir = fs.Arg(0)
		return pullFiles(cfg, terminalConflictPrompt)
	case 2:
		return pullSingleFile(cfg, fs.Arg(0), fs.Arg(1), terminalConflictPrompt)
	default:
		return fmt.Errorf("too many arguments — run 'gossh help pull'")
	}
}

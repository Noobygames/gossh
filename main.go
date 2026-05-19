package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/noobygames/gossh/pkg/config"
	"github.com/noobygames/gossh/pkg/remoteexec"
	"github.com/noobygames/gossh/pkg/transfer"
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
		err = cmdTool("kubectl", os.Args[2:])
	case "helm":
		err = cmdTool("helm", os.Args[2:])
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

func cmdPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	dryRun := fs.Bool("dry-run", false, "List files without transferring")
	var extra stringSlice
	fs.Var(&extra, "exclude", "Additional exclude pattern (repeatable, gitignore-style)")
	fs.Parse(args) //nolint:errcheck // ExitOnError

	fileCfg, err := config.Load()
	if err != nil {
		return err
	}

	opts := transfer.Options{
		Server:    config.FirstNonEmpty(*server, fileCfg.Server),
		RemoteDir: config.FirstNonEmpty(*remoteDir, fileCfg.RemoteDir, "~/kubernetes"),
		DryRun:    *dryRun,
		Identity:  config.FirstNonEmpty(*identity),
		Excludes:  slices.Concat(defaultExcludes, fileCfg.Excludes, []string(extra)),
		Out:       os.Stdout,
	}

	switch fs.NArg() {
	case 0:
		opts.SourceDir = "."
		return transfer.Push(opts)
	case 1:
		opts.SourceDir = fs.Arg(0)
		return transfer.Push(opts)
	case 2:
		localPath, remotePath := fs.Arg(0), fs.Arg(1)
		info, err := os.Stat(localPath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			opts.SourceDir = localPath
			opts.RemoteDir = remotePath
			return transfer.Push(opts)
		}
		return transfer.PushFile(opts, localPath, remotePath)
	default:
		return fmt.Errorf("too many arguments — run 'gossh help push'")
	}
}

func cmdPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	fs.Parse(args) //nolint:errcheck // ExitOnError

	fileCfg, err := config.Load()
	if err != nil {
		return err
	}

	opts := transfer.Options{
		Server:    config.FirstNonEmpty(*server, fileCfg.Server),
		RemoteDir: config.FirstNonEmpty(*remoteDir, fileCfg.RemoteDir, "~/kubernetes"),
		Identity:  config.FirstNonEmpty(*identity),
		Out:       os.Stdout,
	}

	switch fs.NArg() {
	case 0:
		opts.SourceDir = "."
		return transfer.Pull(opts, transfer.TerminalConflictPrompt)
	case 1:
		opts.SourceDir = fs.Arg(0)
		return transfer.Pull(opts, transfer.TerminalConflictPrompt)
	case 2:
		return transfer.PullFile(opts, fs.Arg(0), fs.Arg(1), transfer.TerminalConflictPrompt)
	default:
		return fmt.Errorf("too many arguments — run 'gossh help pull'")
	}
}

func cmdLS(args []string) error {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	server, remoteDir, identity := commonFlags(fs)
	long := fs.Bool("l", false, "Long listing format")
	all := fs.Bool("a", false, "Include hidden entries")
	human := fs.Bool("h", false, "Human-readable sizes (with -l)")
	recursive := fs.Bool("R", false, "Recursive")
	fs.Parse(args) //nolint:errcheck // ExitOnError

	fileCfg, err := config.Load()
	if err != nil {
		return err
	}

	remotePath := config.FirstNonEmpty(*remoteDir, fileCfg.RemoteDir, "~/kubernetes")
	if fs.NArg() > 0 {
		remotePath = fs.Arg(0)
	}

	return transfer.LS(transfer.Options{
		Server:   config.FirstNonEmpty(*server, fileCfg.Server),
		Identity: config.FirstNonEmpty(*identity),
		Out:      os.Stdout,
	}, remotePath, *long, *all, *human, *recursive)
}

func cmdTool(tool string, args []string) error {
	server, identity, toolArgs := remoteexec.ExtractGosshFlags(args)
	fileCfg, err := config.Load()
	if err != nil {
		return err
	}
	return remoteexec.Exec(
		config.FirstNonEmpty(server, fileCfg.Server),
		config.FirstNonEmpty(identity),
		tool,
		toolArgs,
	)
}

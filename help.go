package main

import (
	"flag"
	"fmt"
	"os"
)

func cmdHelp(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "push":
			helpPush()
		case "pull":
			helpPull()
		case "ls":
			helpLS()
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
			helpAll()
		}
		return
	}
	helpAll()
}

func helpAll() {
	fmt.Fprint(os.Stderr, `gossh — sync files and directories over SSH

Usage:
  gossh <command> [flags] [args]

Commands:
  push    Upload a local file or directory to the remote host
  pull    Download a remote file or directory to the local machine
  ls      List files on the remote host
  help    Show help for a command

Examples:
  gossh push -server deploy@host.example.com
  gossh push -server deploy@host.example.com ./file.yaml ~/kubernetes/file.yaml
  gossh pull -server deploy@host.example.com
  gossh pull -server deploy@host.example.com ~/kubernetes/file.yaml ./file.yaml
  gossh ls   -server deploy@host.example.com -l
  gossh help push

Configuration (.gossh.yml in the project or home directory):
  server:     user@host
  remote-dir: ~/kubernetes
  excludes:
    - "*.bak"
    - "dist/"
    - "**/vendor/"

Run "gossh help <command>" for command-specific flags.
`)
}

func helpPush() {
	fmt.Fprint(os.Stderr, `Usage:
  gossh push [flags] [source] [remote-path]

  Uploads a local directory to -remote-dir on the remote host.
  With two path arguments, uploads source (file or directory) to
  the exact remote-path.

Arguments:
  source       Local file or directory to upload (default: .)
  remote-path  Exact remote destination; overrides -remote-dir

Flags:
`)
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	server, remoteDir, identity := commonFlags(fs)
	_ = server
	_ = remoteDir
	_ = identity
	fs.Bool("dry-run", false, "List files without transferring")
	fs.String("exclude", "", "Exclude pattern, gitignore-style (repeatable)")
	fs.PrintDefaults()
}

func helpPull() {
	fmt.Fprint(os.Stderr, `Usage:
  gossh pull [flags] [remote-path local-path]
  gossh pull [flags] [local-dir]

  Downloads the remote directory to a local directory (default: .).
  With two path arguments, downloads a single remote file to local-path.
  Conflicts with existing local files prompt for resolution:
    [s]kip  [S]kip all  [o]verwrite  [O]verwrite all  [a]bort

Arguments:
  local-dir    Local destination for directory pull (default: .)
  remote-path  Exact remote file path (two-argument mode)
  local-path   Local destination path  (two-argument mode)

Flags:
`)
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	server, remoteDir, identity := commonFlags(fs)
	_ = server
	_ = remoteDir
	_ = identity
	fs.PrintDefaults()
}

func helpLS() {
	fmt.Fprint(os.Stderr, `Usage:
  gossh ls [flags] [remote-path]

  Lists files on the remote host. Defaults to -remote-dir.

Arguments:
  remote-path  Remote path to list (default: from config or ~/kubernetes)

Flags:
`)
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	server, remoteDir, identity := commonFlags(fs)
	_ = server
	_ = remoteDir
	_ = identity
	fs.Bool("l", false, "Long listing format")
	fs.Bool("a", false, "Include hidden entries (starting with .)")
	fs.Bool("h", false, "Human-readable file sizes (with -l)")
	fs.Bool("R", false, "List subdirectories recursively")
	fs.PrintDefaults()
}

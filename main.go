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
	"secret.yml",
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
	server := flag.String("server", "", "SSH target (user@host[:port])")
	remoteDir := flag.String("remote-dir", "~/kubernetes", "Remote destination directory")
	dryRun := flag.Bool("dry-run", false, "List files without transferring")
	identity := flag.String("identity", "", "SSH private key (default: ~/.ssh/id_ed25519, ~/.ssh/id_rsa, ~/.ssh/id_ecdsa)")

	var extra stringSlice
	flag.Var(&extra, "exclude", "Additional exclude name (repeatable)")
	flag.Parse()

	sourceDir := "."
	if flag.NArg() > 0 {
		sourceDir = flag.Arg(0)
	}

	cfg := SyncConfig{
		SourceDir: sourceDir,
		Server:    *server,
		RemoteDir: *remoteDir,
		DryRun:    *dryRun,
		Identity:  *identity,
		Excludes:  slices.Concat(defaultExcludes, []string(extra)),
		Out:       os.Stdout,
	}

	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg SyncConfig) error {
	if cfg.DryRun {
		fmt.Fprintf(cfg.Out, "Dry run — would sync: %s → %s:%s\n\n", cfg.SourceDir, cfg.Server, cfg.RemoteDir)
		return listFiles(cfg.Out, cfg.SourceDir, cfg.Excludes)
	}

	fmt.Fprintf(cfg.Out, "Connecting to %s...\n", cfg.Server)
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

	pipe, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	cmd := fmt.Sprintf("mkdir -p %s && tar -xzf - -C %s", cfg.RemoteDir, cfg.RemoteDir)
	if err := sess.Start(cmd); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(cfg.Out, "Syncing %s → %s:%s\n", cfg.SourceDir, cfg.Server, cfg.RemoteDir)

	if err := archiveAndSend(pipe, cfg.Out, cfg.SourceDir, cfg.Excludes); err != nil {
		return err
	}

	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote: %w", err)
	}

	fmt.Fprintln(cfg.Out, "\nDone.")
	return nil
}

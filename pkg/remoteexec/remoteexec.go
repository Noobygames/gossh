package remoteexec

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/noobygames/gossh/pkg/sshconn"
	"github.com/noobygames/gossh/pkg/transfer"
)

// localUpload describes a local file or directory to be uploaded to the remote host.
type localUpload struct {
	localPath  string
	remotePath string
	isDir      bool
}

// ExtractGosshFlags splits args into gossh flags (-server, -identity) and the
// remainder. Parsing stops at the first argument that is not a recognised
// gossh flag, so kubectl/helm arguments can follow without any separator.
func ExtractGosshFlags(args []string) (server, identity string, rest []string) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-server" || args[i] == "--server":
			if i+1 >= len(args) {
				break
			}
			i++
			server = args[i]
		case strings.HasPrefix(args[i], "-server="):
			server = args[i][len("-server="):]
		case strings.HasPrefix(args[i], "--server="):
			server = args[i][len("--server="):]
		case args[i] == "-identity" || args[i] == "--identity":
			if i+1 >= len(args) {
				break
			}
			i++
			identity = args[i]
		case strings.HasPrefix(args[i], "-identity="):
			identity = args[i][len("-identity="):]
		case strings.HasPrefix(args[i], "--identity="):
			identity = args[i][len("--identity="):]
		default:
			rest = args[i:]
			return
		}
	}
	return
}

// Exec uploads any local file/dir arguments to a remote temp directory,
// substitutes paths in args, runs tool with the modified args on the remote
// host, then removes the temp directory.
func Exec(ctx context.Context, server, identity, tool string, args []string) error {
	remoteTempDir := "/tmp/gossh-" + randomHex(8)
	uploads, err := collectLocalUploads(args, remoteTempDir)
	if err != nil {
		return err
	}

	client, err := sshconn.Connect(ctx, server, identity, sshconn.TerminalPrompt)
	if err != nil {
		return err
	}
	defer client.Close()

	if len(uploads) > 0 {
		if err := sshRun(client, "mkdir -p "+shellQuote(remoteTempDir)); err != nil {
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer sshRun(client, "rm -rf "+shellQuote(remoteTempDir)) //nolint:errcheck
		if err := sendUploads(client, os.Stdout, uploads); err != nil {
			return err
		}
	}

	mapping := buildPathMapping(uploads)
	return sshRunInteractive(client, buildRemoteCommand(tool, substituteArgs(args, mapping)))
}

// collectLocalUploads scans args for local paths and returns upload descriptors.
func collectLocalUploads(args []string, remoteTempDir string) ([]localUpload, error) {
	var uploads []localUpload
	for _, arg := range args {
		for _, localPath := range localPathCandidates(arg) {
			u, ok, err := buildUpload(localPath, remoteTempDir)
			if err != nil {
				return nil, err
			}
			if ok {
				uploads = append(uploads, u)
			}
		}
	}
	return uploads, nil
}

func buildUpload(localPath, remoteTempDir string) (localUpload, bool, error) {
	if !isLocalPath(localPath) {
		return localUpload{}, false, nil
	}
	info, err := os.Stat(localPath)
	if err != nil {
		return localUpload{}, false, fmt.Errorf("stat %s: %w", localPath, err)
	}
	return localUpload{
		localPath:  localPath,
		remotePath: remoteTempDir + "/" + filepath.Base(localPath),
		isDir:      info.IsDir(),
	}, true, nil
}

// buildPathMapping constructs a local→remote substitution map from uploads.
func buildPathMapping(uploads []localUpload) map[string]string {
	m := make(map[string]string, len(uploads))
	for _, u := range uploads {
		m[u.localPath] = u.remotePath
	}
	return m
}

// buildRemoteCommand assembles the shell command string for remote execution.
func buildRemoteCommand(tool string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, tool)
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

func localPathCandidates(arg string) []string {
	if !strings.HasPrefix(arg, "-") {
		return []string{arg}
	}
	if _, value, ok := strings.Cut(arg, "="); ok {
		return []string{value}
	}
	return nil
}

func isLocalPath(s string) bool {
	if s == "" || strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return false
	}
	// Only treat as a local path when the argument is unambiguously a path:
	// must start with ./, ../, or be absolute. Bare names like "raid-assignments"
	// are tool arguments (pod names, release names, …) even if a same-named local
	// entry happens to exist.
	if !strings.HasPrefix(s, "./") && !strings.HasPrefix(s, "../") && !filepath.IsAbs(s) {
		return false
	}
	_, err := os.Stat(s)
	return err == nil
}

func substituteArgs(args []string, mapping map[string]string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = substituteArg(arg, mapping)
	}
	return out
}

func substituteArg(arg string, mapping map[string]string) string {
	if remote, ok := mapping[arg]; ok {
		return remote
	}
	if !strings.HasPrefix(arg, "-") {
		return arg
	}
	key, value, ok := strings.Cut(arg, "=")
	if !ok {
		return arg
	}
	if remote, found := mapping[value]; found {
		return key + "=" + remote
	}
	return arg
}

func sendUploads(client *ssh.Client, out io.Writer, uploads []localUpload) error {
	for _, u := range uploads {
		if err := sendUpload(client, out, u); err != nil {
			return fmt.Errorf("upload %s: %w", u.localPath, err)
		}
	}
	return nil
}

func sendUpload(client *ssh.Client, out io.Writer, u localUpload) error {
	if u.isDir {
		return uploadDir(client, out, u.localPath, u.remotePath)
	}
	return uploadFile(client, out, u.localPath, u.remotePath)
}

func uploadFile(client *ssh.Client, out io.Writer, localPath, remotePath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	defer sess.Close()
	sess.Stderr = os.Stderr

	pipe, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	remoteDir := shellQuote(path.Dir(remotePath))
	remoteFile := shellQuote(remotePath)
	if err := sess.Start(fmt.Sprintf("mkdir -p %s && cat > %s", remoteDir, remoteFile)); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(out, "  upload  %s (%d B)\n", localPath, info.Size())
	if _, err := io.Copy(pipe, f); err != nil {
		return err
	}
	if err := pipe.Close(); err != nil {
		return err
	}
	return sess.Wait()
}

func uploadDir(client *ssh.Client, out io.Writer, localPath, remotePath string) error {
	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	defer sess.Close()
	sess.Stderr = os.Stderr

	pipe, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	quoted := shellQuote(remotePath)
	if err := sess.Start(fmt.Sprintf("mkdir -p %s && tar -xzf - -C %s", quoted, quoted)); err != nil {
		return fmt.Errorf("remote start: %w", err)
	}

	fmt.Fprintf(out, "  upload  %s/\n", localPath)
	if err := transfer.ArchiveAndSend(pipe, out, localPath, nil); err != nil {
		return err
	}
	return sess.Wait()
}

func sshRun(client *ssh.Client, cmd string) error {
	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	defer sess.Close()
	sess.Stderr = os.Stderr
	return sess.Run(cmd)
}

func sshRunInteractive(client *ssh.Client, cmd string) error {
	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	defer sess.Close()

	sess.Stdin = os.Stdin
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr

	requestPtyIfTerminal(sess)
	return sess.Run(cmd)
}

func requestPtyIfTerminal(sess *ssh.Session) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		w, h = 80, 24
	}
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 38400, ssh.TTY_OP_OSPEED: 38400}
	sess.RequestPty("xterm-256color", h, w, modes) //nolint:errcheck
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read: %v", err))
	}
	return hex.EncodeToString(b)
}

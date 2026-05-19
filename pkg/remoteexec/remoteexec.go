package remoteexec

import (
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

// ExtractGosshFlags splits args into gossh flags (-server, -identity) and the
// remainder. Parsing stops at the first argument that is not a recognised
// gossh flag, so kubectl/helm arguments can follow without any separator.
func ExtractGosshFlags(args []string) (server, identity string, rest []string) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-server" || args[i] == "--server":
			if i+1 < len(args) {
				server = args[i+1]
				i++
			}
		case strings.HasPrefix(args[i], "-server="):
			server = args[i][len("-server="):]
		case strings.HasPrefix(args[i], "--server="):
			server = args[i][len("--server="):]
		case args[i] == "-identity" || args[i] == "--identity":
			if i+1 < len(args) {
				identity = args[i+1]
				i++
			}
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
func Exec(server, identity, tool string, args []string) error {
	type upload struct {
		localPath  string
		remotePath string
		isDir      bool
	}

	var uploads []upload
	remoteTempDir := ""

	for _, arg := range args {
		for _, localPath := range localPathCandidates(arg) {
			if !isLocalPath(localPath) {
				continue
			}
			if remoteTempDir == "" {
				remoteTempDir = "/tmp/gossh-" + randomHex(8)
			}
			info, err := os.Stat(localPath)
			if err != nil {
				return fmt.Errorf("stat %s: %w", localPath, err)
			}
			uploads = append(uploads, upload{
				localPath:  localPath,
				remotePath: remoteTempDir + "/" + filepath.Base(localPath),
				isDir:      info.IsDir(),
			})
		}
	}

	client, err := sshconn.Connect(server, identity, sshconn.TerminalPrompt)
	if err != nil {
		return err
	}
	defer client.Close()

	if len(uploads) > 0 {
		if err := sshRun(client, fmt.Sprintf("mkdir -p %s", remoteTempDir)); err != nil {
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer sshRun(client, fmt.Sprintf("rm -rf %s", remoteTempDir)) //nolint:errcheck

		for _, u := range uploads {
			if u.isDir {
				if err := uploadDir(client, os.Stdout, u.localPath, u.remotePath); err != nil {
					return fmt.Errorf("upload %s: %w", u.localPath, err)
				}
			} else {
				if err := uploadFile(client, os.Stdout, u.localPath, u.remotePath); err != nil {
					return fmt.Errorf("upload %s: %w", u.localPath, err)
				}
			}
		}
	}

	mapping := make(map[string]string, len(uploads))
	for _, u := range uploads {
		mapping[u.localPath] = u.remotePath
	}

	remoteArgs := substituteArgs(args, mapping)
	parts := make([]string, 0, len(remoteArgs)+1)
	parts = append(parts, tool)
	for _, a := range remoteArgs {
		parts = append(parts, shellQuote(a))
	}
	return sshRunInteractive(client, strings.Join(parts, " "))
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
	_, err := os.Stat(s)
	return err == nil
}

func substituteArgs(args []string, mapping map[string]string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		if remote, ok := mapping[arg]; ok {
			out[i] = remote
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if key, value, ok := strings.Cut(arg, "="); ok {
				if remote, found := mapping[value]; found {
					out[i] = key + "=" + remote
					continue
				}
			}
		}
		out[i] = arg
	}
	return out
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

	if err := sess.Start(fmt.Sprintf("mkdir -p %s && cat > %s", path.Dir(remotePath), remotePath)); err != nil {
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

	if err := sess.Start(fmt.Sprintf("mkdir -p %s && tar -xzf - -C %s", remotePath, remotePath)); err != nil {
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

	if term.IsTerminal(int(os.Stdin.Fd())) {
		w, h, err := term.GetSize(int(os.Stdin.Fd()))
		if err != nil {
			w, h = 80, 24
		}
		modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 38400, ssh.TTY_OP_OSPEED: 38400}
		if err := sess.RequestPty("xterm-256color", h, w, modes); err != nil {
			// non-fatal: proceed without PTY
		}
	}

	return sess.Run(cmd)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

package sshconn

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

// PassphrasePrompt is called when a private key requires a passphrase.
type PassphrasePrompt func(keyPath string) ([]byte, error)

// TerminalPrompt reads a passphrase interactively from the terminal.
func TerminalPrompt(keyPath string) ([]byte, error) {
	fmt.Fprintf(os.Stderr, "Passphrase for %s: ", keyPath)
	p, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return p, err
}

// Connect dials server (user@host[:port]) using key authentication.
// identity specifies a key path; if empty, standard ~/.ssh locations are tried.
// An SSH agent (ssh-agent / Windows OpenSSH Agent) is tried first so no passphrase prompt is needed.
func Connect(ctx context.Context, server, identity string, prompt PassphrasePrompt) (*ssh.Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home dir: %w", err)
	}

	var agentMethod ssh.AuthMethod
	if rwc := dialAgent(); rwc != nil {
		defer rwc.Close()
		agentMethod = ssh.PublicKeysCallback(agent.NewClient(rwc).Signers)
	}

	auth, err := loadAuth(identity, home, prompt, agentMethod)
	if err != nil {
		return nil, err
	}
	if len(auth) == 0 {
		return nil, ErrNoAuth
	}
	user, host, err := ParseTarget(server)
	if err != nil {
		return nil, err
	}
	khPath := filepath.Join(home, ".ssh", "known_hosts")
	hkc, err := knownhosts.New(khPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &KnownHostsNotFoundError{Path: khPath, Host: host}
	}
	if err != nil {
		return nil, fmt.Errorf("known_hosts: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hkc,
	}
	conn, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, host, cfg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake %s: %w", host, err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// ParseTarget splits "user@host[:port]" into user and host:port.
func ParseTarget(server string) (user, host string, err error) {
	parts := strings.SplitN(server, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", &InvalidServerError{Server: server}
	}
	user, host = parts[0], parts[1]
	if !strings.Contains(host, ":") {
		host += ":22"
	}
	return user, host, nil
}

func loadAuth(identity, home string, prompt PassphrasePrompt, agentMethod ssh.AuthMethod) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if agentMethod != nil {
		methods = append(methods, agentMethod)
	}
	if identity != "" {
		m, err := keyAuth(identity, prompt)
		if err != nil {
			return nil, err
		}
		if m == nil {
			return nil, &IdentityNotFoundError{Path: identity}
		}
		return append(methods, m), nil
	}
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		m, err := keyAuth(filepath.Join(home, ".ssh", name), prompt)
		if err != nil {
			return nil, err
		}
		if m != nil {
			methods = append(methods, m)
		}
	}
	return methods, nil
}

func keyAuth(path string, prompt PassphrasePrompt) (ssh.AuthMethod, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read key %s: %w", path, err)
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err == nil {
		return ssh.PublicKeys(signer), nil
	}
	var passErr *ssh.PassphraseMissingError
	if !errors.As(err, &passErr) {
		return nil, fmt.Errorf("parse key %s: %w", path, err)
	}
	passphrase, err := prompt(path)
	if err != nil {
		return nil, fmt.Errorf("reading passphrase: %w", err)
	}
	signer, err = ssh.ParsePrivateKeyWithPassphrase(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("wrong passphrase for %s: %w", path, err)
	}
	return ssh.PublicKeys(signer), nil
}

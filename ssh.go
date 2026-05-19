package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

type PassphrasePrompt func(keyPath string) ([]byte, error)

func terminalPrompt(keyPath string) ([]byte, error) {
	fmt.Fprintf(os.Stderr, "Passphrase for %s: ", keyPath)
	p, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return p, err
}

func connect(cfg SyncConfig, prompt PassphrasePrompt) (*ssh.Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home dir: %w", err)
	}
	auth, err := loadAuth(cfg.Identity, home, prompt)
	if err != nil {
		return nil, err
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("no SSH auth: use -identity or ensure ~/.ssh/id_ed25519 or ~/.ssh/id_rsa exists")
	}
	user, host, err := parseTarget(cfg.Server)
	if err != nil {
		return nil, err
	}
	hkc, err := knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
	if err != nil {
		return nil, fmt.Errorf("known_hosts: %w", err)
	}
	client, err := ssh.Dial("tcp", host, &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hkc,
	})
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	return client, nil
}

func parseTarget(server string) (user, host string, err error) {
	parts := strings.SplitN(server, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid server %q: expected user@host", server)
	}
	user, host = parts[0], parts[1]
	if !strings.Contains(host, ":") {
		host += ":22"
	}
	return user, host, nil
}

func loadAuth(identity, home string, prompt PassphrasePrompt) ([]ssh.AuthMethod, error) {
	if identity != "" {
		m, err := keyAuth(identity, prompt)
		if err != nil {
			return nil, err
		}
		return []ssh.AuthMethod{m}, nil
	}
	var methods []ssh.AuthMethod
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
	if err != nil {
		return nil, nil // key absent: skip silently
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err == nil {
		return ssh.PublicKeys(signer), nil
	}
	if _, ok := err.(*ssh.PassphraseMissingError); !ok {
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


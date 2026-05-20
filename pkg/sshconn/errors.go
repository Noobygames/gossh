package sshconn

import "fmt"

// ErrNoAuth is returned when no SSH authentication method could be loaded.
var ErrNoAuth = fmt.Errorf("no SSH auth: use -identity or ensure ~/.ssh/id_ed25519 or ~/.ssh/id_rsa exists")

// InvalidServerError is returned when the server string is not in user@host[:port] format.
type InvalidServerError struct {
	Server string
}

func (e *InvalidServerError) Error() string {
	return fmt.Sprintf("invalid server %q: expected user@host[:port]", e.Server)
}

// IdentityNotFoundError is returned when an explicitly provided identity file does not exist.
type IdentityNotFoundError struct {
	Path string
}

func (e *IdentityNotFoundError) Error() string {
	return fmt.Sprintf("identity file not found: %s", e.Path)
}

// KnownHostsNotFoundError is returned when the known_hosts file is missing.
// Path is the expected file location; Host is the target that triggered the lookup.
type KnownHostsNotFoundError struct {
	Path string
	Host string
}

func (e *KnownHostsNotFoundError) Error() string {
	return fmt.Sprintf("%s not found: connect to %s with plain 'ssh' once to register the host key", e.Path, e.Host)
}

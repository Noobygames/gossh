//go:build !windows

package sshconn

import (
	"io"
	"net"
	"os"
)

// dialAgent connects to the SSH agent via SSH_AUTH_SOCK. Returns nil if no agent is running.
func dialAgent() io.ReadWriteCloser {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	return conn
}

//go:build windows

package sshconn

import (
	"io"
	"os"
)

// dialAgent connects to the Windows OpenSSH Agent via its named pipe. Returns nil if no agent is running.
func dialAgent() io.ReadWriteCloser {
	pipe, err := os.OpenFile(`\\.\pipe\openssh-ssh-agent`, os.O_RDWR, 0)
	if err != nil {
		return nil
	}
	return pipe
}

package sshagent

import (
	"golang.org/x/crypto/ssh/agent"
)

// Agent wraps agent.ExtendedAgent
type Agent struct {
	agent.ExtendedAgent
}

// NewAgent connects to a local SSH agent either via unix socket or named pipes depending on the OS
//
// In the case of a non-Windows OS, the "SSH_AUTH_SOCK" environment variable must be set or the
// process will fail.
//
// On Windows a connection is attempted to "\\.\pipe\openssh-ssh-agent" which is the default
// named pipe path for the native OpenSSH Authentication Agent.
func NewAgent() (*Agent, error) {
	conn, err := connect()
	if err != nil {
		return nil, err
	}

	client := agent.NewClient(conn)

	return &Agent{client}, nil
}

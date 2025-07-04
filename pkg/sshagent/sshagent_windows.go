package sshagent

import (
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh/agent"
)

func connect() (net.Conn, error) {
	return winio.DialPipe("\\\\.\\pipe\\openssh-ssh-agent", nil)
}

// On Windows the native "ssh-agent" implementation does not support adding keys with
// [agent.Addedkey.LifetimeSecs] or [agent.AddedKey.ConfirmBeforeUse] so these are
// set to zero/false before adding the key.
func (a Agent) Add(key agent.AddedKey) error {
	key.ConfirmBeforeUse = false
	key.LifetimeSecs = 0

	return a.ExtendedAgent.Add(key)
}

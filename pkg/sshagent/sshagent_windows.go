package sshagent

import (
	"net"
	
	"github.com/Microsoft/go-winio"
)

func connect() (net.Conn, error) {
	return winio.DialPipe("\\\\.\\pipe\\openssh-ssh-agent", nil)
}

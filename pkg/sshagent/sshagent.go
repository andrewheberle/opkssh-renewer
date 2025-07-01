package sshagent

import (
	"net"
)

type Connection struct {
	net.Conn
}

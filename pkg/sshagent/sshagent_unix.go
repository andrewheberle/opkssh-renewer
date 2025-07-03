// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris

package sshagent

import (
	"net"
	"os"
)

func connect() (net.Conn, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	return net.Dial("unix", socket)
}

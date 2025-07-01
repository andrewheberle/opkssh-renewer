package sshagent

import (
	"net"
	"os"
)

func Connect() (*Connection, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, err
	}

	return &Connection{conn}, nil
}

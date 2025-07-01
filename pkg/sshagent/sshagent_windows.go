package sshagent

import "github.com/Microsoft/go-winio"

func Connect() (*Connection, error) {
	conn, err := winio.DialPipe("\\\\.\\pipe\\openssh-ssh-agent", nil)
	if err != nil {
		return nil, err
	}

	return &Connection{conn}, nil
}

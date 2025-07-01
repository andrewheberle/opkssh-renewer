package sshagent

import (
	"golang.org/x/crypto/ssh/agent"
)

type Agent struct {
	agent.ExtendedAgent
}

func NewAgent() (*Agent, error) {
	conn, err := connect()
	if err != nil {
		return err
	}

	client := agent.NewClient(conn)

	return &Agent{client}, nil
}

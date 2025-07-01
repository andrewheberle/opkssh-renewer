package sshagent

import (
	"golang.org/x/crypto/ssh/agent"
)

type Agent struct {
	agent.ExtendedAgent
}

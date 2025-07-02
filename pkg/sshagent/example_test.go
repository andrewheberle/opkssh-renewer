package sshagent_test

import (
	"fmt"

	"github.com/andrewheberle/opkssh-renewer/pkg/sshagent"
)

// This example shows connecting to an existing agent and listing all identities
// from it.
//
// This will fail unless a local SSH agent is running
func ExampleNewAgent() {
	client, err := sshagent.NewAgent()
	if err != nil {
		fmt.Printf("error connecting to agent")

		return
	}

	if _, err := client.List(); err != nil {
		fmt.Printf("error listing keys agent")
	}
	// Output:
	// error connecting to agent
}

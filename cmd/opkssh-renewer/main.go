package main

import (
	"log/slog"
	"os"

	"github.com/andrewheberle/opkssh-renewer/internal/pkg/cmd"
)

func main() {
	if err := cmd.Execute(os.Args[1:]); err != nil {
		slog.Error("Error during execution", "error", err)
		os.Exit(1)
	}
}

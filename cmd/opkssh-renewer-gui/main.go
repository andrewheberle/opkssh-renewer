package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/andrewheberle/opkssh-renewer/pkg/gui"
)

//go:embed icons
var resources embed.FS

func main() {
	appname := "OpkSSH Renewer"
	a, err := gui.Create(appname, resources, "id_opkssh")
	if err != nil {
		slog.Error("Error during execution", "error", err)
		os.Exit(1)
	}
	a.Run()
}

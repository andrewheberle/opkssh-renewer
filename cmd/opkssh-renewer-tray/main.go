package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/andrewheberle/opkssh-renewer/internal/pkg/tray"
	"github.com/gen2brain/beeep"
)

//go:embed icons
var resources embed.FS

func main() {
	beeep.AppName = "OpkSSH Renewer"

	app, err := tray.New(beeep.AppName, resources)
	if err != nil {
		slog.Error("Error during execution", "error", err)
		os.Exit(1)

	}
	app.Run()
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func loadIcon(name string) (fyne.Resource, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}

	return fyne.NewStaticResource(filepath.Base(name), data), nil
}

func providerRow(name string, icon fyne.Resource, tapped func()) *fyne.Container {
	return container.New(
		layout.NewHBoxLayout(),
		widget.NewLabel(name),
		widget.NewLabel(time.Since(time.Now().Add(-(time.Hour * 2))).String()),
		widget.NewButtonWithIcon("", icon, tapped),
	)
}

func main() {
	appname := "OpkSSH Renewer"
	a := app.New()

	// load icon
	icon, err := loadIcon("icons/key.png")
	if err != nil {
		fmt.Printf("Error loading icon: %s", err)
		os.Exit(1)
	}
	a.SetIcon(icon)

	w := a.NewWindow(appname)

	// set up system tray
	if desk, ok := a.(desktop.App); ok {
		// create menu
		trayMenu := fyne.NewMenu(appname,
			fyne.NewMenuItem("Show", func() {
				w.Show()
			}))
		desk.SetSystemTrayMenu(trayMenu)
		desk.SetSystemTrayIcon(icon)

		// make close just hide to tray
		w.SetCloseIntercept(func() {
			w.Hide()
		})
	}

	// load refresh button
	refresh, err := loadIcon("icons/refresh.png")
	if err != nil {
		fmt.Printf("Error loading icon: %s", err)
		os.Exit(1)
	}

	// build main window
	w.SetContent(
		container.New(
			layout.NewVBoxLayout(),
			widget.NewLabel("Providers"),
			providerRow("cloudflare", refresh, func() {}),
		),
	)
	w.ShowAndRun()
}

package gui

import (
	"embed"
	"fmt"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/andrewheberle/opkssh-renewer/pkg/opkssh"
)

func Create(appname string, fs embed.FS, identity string) (fyne.App, error) {
	a := app.New()

	// load icon
	icon, err := loadIcon(fs, "icons/key.png")
	if err != nil {
		return nil, fmt.Errorf("error loading icon: %w", err)
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
	refresh, err := loadIcon(fs, "icons/refresh.png")
	if err != nil {
		return nil, fmt.Errorf("error loading icon: %w", err)

	}

	// set up renewer
	renewer, err := opkssh.NewRenewer(identity, time.Hour*23, false)
	if err != nil {
		return nil, fmt.Errorf("error setting up renewer: %w", err)
	}

	// build main window
	w.SetContent(
		container.New(
			layout.NewVBoxLayout(),
			widget.NewLabel("Identity"),
			identityRow(
				renewer.Name(),
				renewer.IdentityAge(),
				refresh,
				func() {
					renewer.Run()
				}),
		),
	)
	w.Show()

	return a, nil
}

func loadIcon(fs embed.FS, name string) (fyne.Resource, error) {
	data, err := fs.ReadFile(name)
	if err != nil {
		return nil, err
	}

	return fyne.NewStaticResource(filepath.Base(name), data), nil
}

func identityRow(name string, age time.Duration, icon fyne.Resource, tapped func()) *fyne.Container {
	return container.New(
		layout.NewHBoxLayout(),
		widget.NewLabel(name),
		widget.NewLabel(fmt.Sprintf("%02dh%02dm", int(age.Hours()), int(age.Minutes())%60)),
		widget.NewButtonWithIcon("", icon, tapped),
	)
}

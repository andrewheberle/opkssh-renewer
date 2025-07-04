package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

func main() {
	appname := "Hello World"
	a := app.New()
	w := a.NewWindow(appname)

	// create menu
	trayMenu := fyne.NewMenu(appname,
		fyne.NewMenuItem("Show", func() {
			w.Show()
		}))

	// set up system tray
	desk, isDesktopApp := a.(desktop.App)
	if isDesktopApp {
		desk.SetSystemTrayMenu(trayMenu)
		w.SetCloseIntercept(func() {
			// make sure close actually closes
			a.Quit()
		})
	}

	w.SetContent(widget.NewLabel("Hello World!"))
	w.ShowAndRun()
}

package gui

import (
	"embed"
	"fmt"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"fyne.io/systray"
	"github.com/andrewheberle/opkssh-renewer/internal/pkg/opkssh"
)

type App struct {
	// notification tracking
	expiryMissing bool
	expiryPassed  bool
	expiryClose   bool

	// bindings
	statusText   binding.String
	age          binding.String
	forceRenewal binding.Bool

	// labels
	ageLabel    *widget.Label
	statusLabel *widget.Label
	titleLabel  *widget.Label

	// widgets
	forceCheck  *widget.Check
	renewButton *widget.Button

	renewer *opkssh.Renewer

	app    fyne.App
	window fyne.Window
}

func Create(appname string, fs embed.FS, identity string) (*App, error) {

	a := new(App)
	a.app = app.New()
	// load icon
	icon, err := loadIcon(fs, "icons/app.png")
	if err != nil {
		return nil, fmt.Errorf("error loading icon: %w", err)
	}
	a.app.SetIcon(icon)

	// set up window
	a.window = a.app.NewWindow(appname)
	a.window.SetFixedSize(true)

	// set up system tray
	a.setSystemTrayMenu(
		fyne.NewMenu(appname,
			fyne.NewMenuItem("Show", func() {
				a.window.Show()
			}),
			fyne.NewMenuItem("Renew", func() {
				a.renew()
			}),
		))
	a.setSystemTrayIcon(icon)

	// set up opkssh renewer
	renewer, err := opkssh.NewRenewer(identity, time.Hour*23)
	if err != nil {
		return nil, fmt.Errorf("error setting up renewer: %w", err)
	}
	a.renewer = renewer

	// bindings
	a.statusText = binding.NewString()
	a.age = binding.NewString()
	a.forceRenewal = binding.NewBool()

	// labels
	a.ageLabel = widget.NewLabelWithData(a.age)
	a.statusLabel = widget.NewLabelWithData(a.statusText)
	a.titleLabel = widget.NewLabel("Identity")
	a.titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// set default systray tooltip
	a.app.Lifecycle().SetOnStarted(func() {
		a.setsystraytooltip()
		a.setagelabel()
	})

	// force checkbox
	a.forceCheck = widget.NewCheckWithData("Force?", a.forceRenewal)

	// renew button
	renewIcon, err := loadIcon(fs, "icons/refresh.png")
	if err != nil {
		return nil, fmt.Errorf("error loading icon: %w", err)
	}
	a.renewButton = widget.NewButtonWithIcon("Renew Identity", renewIcon, func() {})
	a.renewButton.OnTapped = a.renew

	// refresh age
	go func() {
		for {
			// sleep for a minute
			time.Sleep(time.Minute)

			fyne.Do(a.setagelabel)
			a.setsystraytooltip()

			identityAge := renewer.IdentityAge()

			// is identity missing
			if identityAge == -1 {
				// set status
				a.statusText.Set("No identity found")

				// send notification
				if !a.expiryMissing {
					a.notification("Identity missing", "No SSH identity was found, please renew")
					a.expiryMissing = true
				}
				continue
			}

			// have we expired?
			if identityAge > time.Hour*24 {
				// set status
				a.statusText.Set("Current identity has expired")

				// send notification
				if !a.expiryPassed {
					a.notification("Identity expired", "The SSH identity has expired and should be renewed")
					a.expiryPassed = true
				}
				continue
			}

			// are we close to expiry
			if identityAge > time.Hour*23 {
				// set status
				a.statusText.Set("Current identity is close to expiry")

				// send notification
				if !a.expiryClose {
					a.notification("Identity nearly expired", "The SSH identity is close to expiry and should be renewed soon")
					a.expiryClose = true
				}
				continue
			}

			// set status
			a.statusText.Set("Current identity valid, no action required")
		}
	}()

	// build main window
	a.window.SetContent(a.content())
	a.window.Show()

	return a, nil
}

func (a *App) Run() {
	a.app.Run()
}

func (a *App) renew() {
	force, err := a.forceRenewal.Get()
	if err != nil {
		a.statusText.Set(fmt.Sprintf("Error reading force status: %s", err))
		return
	}

	if a.renewer.IdentityAge() < time.Hour*23 && a.renewer.IdentityAge() != -1 {
		if !force {
			a.statusText.Set("Renewal not required yet")
			a.notification("Not Required", "Renewal not required as identity has more than 23-hours left until expiry")
			return
		}

		a.statusText.Set("Renewal forced...")
	}

	a.statusText.Set("Starting renewal...")

	go func() {
		fyne.Do(func() {
			// disable the button and checkbox
			a.renewButton.Disable()
			a.forceCheck.Disable()
		})
		defer fyne.Do(func() {
			// enable button and checkbox again
			a.forceCheck.Enable()
			a.renewButton.Enable()
		})

		// run renewal
		if force {
			err = a.renewer.ForceRenew()
		} else {
			err = a.renewer.Renew()
		}
		if err != nil {
			fyne.Do(func() {
				a.statusText.Set(fmt.Sprintf("Error during identity refresh: %s", err))
			})
			a.notification("Error", fmt.Sprintf("Error during identity refresh: %s", err))
			return
		}

		// update stuff post renewal
		fyne.Do(func() {
			a.statusText.Set("Identity renewed")
			a.setagelabel()
			a.forceRenewal.Set(false)
		})
		a.notification("Identity Renewed", "The identity was successfully renewed")

		// set tooltip on systray
		a.setsystraytooltip()

		// reset notification status
		a.expiryMissing = false
		a.expiryClose = false
		a.expiryPassed = false
	}()
}

func (a *App) setsystraytooltip() {
	identityAge := a.renewer.IdentityAge()

	if identityAge == -1 {
		systray.SetTooltip("Current identity missing")
		return
	}

	systray.SetTooltip(fmt.Sprintf("Current identity has %s until expiry", formatDuration((time.Hour*24)-identityAge)))
}

func (a *App) setagelabel() {
	identityAge := a.renewer.IdentityAge()

	if identityAge == -1 {
		a.age.Set("missing")
		a.ageLabel.Importance = widget.DangerImportance
		return
	}

	// expired
	if identityAge > time.Hour*24 {
		a.age.Set("expired")
		a.ageLabel.Importance = widget.DangerImportance
		return
	}

	// set standard age label text and importanc
	a.age.Set(formatDuration((time.Hour * 24) - a.renewer.IdentityAge()))
	a.ageLabel.Importance = widget.MediumImportance

	if identityAge > time.Hour*23 {
		a.ageLabel.Importance = widget.WarningImportance
	}
}

func (a *App) notification(title, content string) {
	a.app.SendNotification(
		fyne.NewNotification(title, content),
	)
}

func (a *App) content() *fyne.Container {
	return container.New(
		layout.NewVBoxLayout(),
		a.titleLabel,
		container.New(
			layout.NewHBoxLayout(),
			widget.NewLabel(a.renewer.Name()),
			a.ageLabel,
			a.renewButton,
			a.forceCheck,
		),
		widget.NewSeparator(),
		a.statusLabel,
	)
}

func (a *App) setSystemTrayIcon(icon fyne.Resource) {
	// set up system tray icon
	if desk, ok := a.app.(desktop.App); ok {
		desk.SetSystemTrayIcon(icon)
	}
}

func (a *App) setSystemTrayMenu(menu *fyne.Menu) {
	// set up system tray
	if desk, ok := a.app.(desktop.App); ok {
		desk.SetSystemTrayMenu(menu)

		// make close just hide to tray
		a.window.SetCloseIntercept(func() {
			a.window.Hide()
		})
	}
}

func loadIcon(fs embed.FS, name string) (fyne.Resource, error) {
	data, err := fs.ReadFile(name)
	if err != nil {
		return nil, err
	}

	return fyne.NewStaticResource(filepath.Base(name), data), nil
}

func formatDuration(d time.Duration) string {
	if d == -1 {
		return "missing"
	}

	return fmt.Sprintf("%02dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

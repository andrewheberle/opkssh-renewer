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
	"fyne.io/fyne/v2/theme"
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
	statusText       binding.String
	age              binding.String
	forceRenewal     binding.Bool
	settingsIdentity binding.String
	identity         binding.String
	startHidden      binding.Bool

	// labels
	ageLabel      *widget.Label
	identityLabel *widget.Label
	statusLabel   *widget.Label
	titleLabel    *widget.Label

	// widgets
	forceCheck     *widget.Check
	renewButton    *widget.Button
	settingsButton *widget.Button
	settingsPopup  *widget.PopUp

	// channel to signal we are done
	done chan bool

	renewer *opkssh.Renewer

	app        fyne.App
	mainWindow fyne.Window
}

func Create(appname string, fs embed.FS) (*App, error) {
	a := new(App)
	a.app = app.New()
	// load icon
	icon, err := loadIcon(fs, "icons/app.png")
	if err != nil {
		return nil, fmt.Errorf("error loading icon: %w", err)
	}
	a.app.SetIcon(icon)

	// set up window
	a.mainWindow = a.app.NewWindow(appname)
	a.mainWindow.SetFixedSize(true)

	// set up system tray
	a.setSystemTrayMenu(
		fyne.NewMenu(appname,
			fyne.NewMenuItem("Show", func() {
				a.mainWindow.Show()
			}),
			fyne.NewMenuItem("Renew", func() {
				a.renew()
			}),
		))
	a.setSystemTrayIcon(icon)

	// get identity from preferences
	identity := a.app.Preferences().StringWithFallback("identity", "id_opkssh")
	startHidden := a.app.Preferences().BoolWithFallback("startHidden", false)

	// set up opkssh renewer
	renewer, err := opkssh.NewRenewer(identity, time.Hour*23)
	if err != nil {
		return nil, fmt.Errorf("error setting up renewer: %w", err)
	}
	a.renewer = renewer

	// set up channel
	a.done = make(chan bool)

	// bindings
	a.statusText = binding.NewString()
	a.age = binding.NewString()
	a.forceRenewal = binding.NewBool()
	a.settingsIdentity = binding.NewString()
	a.settingsIdentity.Set(identity)
	a.identity = binding.NewString()
	a.identity.Set(identity)
	a.startHidden = binding.NewBool()
	a.startHidden.Set(startHidden)

	// labels
	a.ageLabel = widget.NewLabelWithData(a.age)
	a.identityLabel = widget.NewLabelWithData(a.identity)
	a.statusLabel = widget.NewLabelWithData(a.statusText)
	a.statusLabel.Truncation = fyne.TextTruncateEllipsis
	a.titleLabel = widget.NewLabel("Identity")
	a.titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// set up hooks for start/stop
	a.app.Lifecycle().SetOnStarted(func() {
		a.setsystraytooltip()
		a.setagelabel()

		// do we start hidden
		if startHidden {
			a.mainWindow.Hide()
			a.notification("Started", "The application is running in the background")
		}
	})
	a.app.Lifecycle().SetOnStopped(func() {
		// stop background task
		a.done <- true
	})

	// force checkbox
	a.forceCheck = widget.NewCheckWithData("Force?", a.forceRenewal)

	// renew button
	a.renewButton = widget.NewButtonWithIcon("Renew Identity", theme.HistoryIcon(), func() {})
	a.renewButton.OnTapped = a.renew

	// settings modal
	a.createSettingsPopup()

	// update status and refresh stuff every minute in the background
	go a.update(time.Minute)

	// build windows
	a.mainWindow.SetContent(a.content())

	// resize a bit bigger than content in case things change
	a.mainWindow.Show()
	a.mainWindow.Resize(fyne.Size{
		Width:  a.mainWindow.Content().Size().Width + 200,
		Height: a.mainWindow.Content().Size().Height,
	})

	return a, nil
}

func (a *App) Run() {
	a.app.Run()
}

func (a *App) update(sleep time.Duration) {
	t := time.NewTicker(sleep)
	defer t.Stop()

	for {
		select {
		case <-a.done:
			return
		case <-t.C:
			fyne.Do(a.setagelabel)
			a.setsystraytooltip()

			identityAge := a.renewer.IdentityAge()

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
	}
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

	if identityAge > time.Hour*24 {
		systray.SetTooltip("Current identity has expired")
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
	// row to show identity name, age, renew and force
	identityRow := container.New(
		layout.NewHBoxLayout(),
		a.identityLabel,
		layout.NewSpacer(),
		a.ageLabel,
		a.renewButton,
		a.forceCheck,
	)
	return container.New(
		layout.NewVBoxLayout(),
		container.New(
			layout.NewHBoxLayout(),
			a.titleLabel,
			layout.NewSpacer(),
			a.settingsButton,
		),
		identityRow,
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
		a.mainWindow.SetCloseIntercept(func() {
			a.mainWindow.Hide()
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

func (a *App) createSettingsPopup() {
	// entry widget
	entry := widget.NewEntryWithData(a.settingsIdentity)

	// hidden
	hiddenCheck := widget.NewCheckWithData("", a.startHidden)

	a.settingsPopup = widget.NewModalPopUp(
		&widget.Form{
			Items: []*widget.FormItem{
				{Text: "Identity Name", Widget: entry},
				{Text: "Start Hidden", Widget: hiddenCheck},
			},
			OnSubmit: func() {
				// always close
				defer a.settingsPopup.Hide()

				// save to preferences
				v, err := a.settingsIdentity.Get()
				if err != nil {
					a.notification("Error", "There was a problem saving your settings")
					return
				}

				current, err := a.identity.Get()
				if err != nil {
					a.notification("Error", "There was a problem saving your settings")
					return
				}

				// does renewer need recreating?
				if v != current {
					renewer, err := opkssh.NewRenewer(v, time.Hour*23)
					if err != nil {
						a.notification("Error", "There was a problem setting up the new renewer")
						return
					}

					a.renewer = renewer

					// set ui stuff based on new renewer
					a.setsystraytooltip()
					a.setagelabel()
				}

				// set in preferences and current value
				a.app.Preferences().SetString("identity", v)
				a.identity.Set(v)

				// save value of startHidden
				h, err := a.startHidden.Get()
				if err != nil {
					a.notification("Error", "There was a problem saving your settings")
					return
				}
				a.app.Preferences().SetBool("startHidden", h)
			},
			OnCancel: func() {
				defer a.settingsPopup.Hide()

				// set back to value from preferences
				a.settingsIdentity.Set(a.app.Preferences().StringWithFallback("identity", "id_opkssh"))
				a.startHidden.Set(a.app.Preferences().BoolWithFallback("startHidden", false))
			},
		},
		a.mainWindow.Canvas(),
	)

	// set up settings button to show modal
	a.settingsButton = widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		size := fyne.Size{
			Width:  a.mainWindow.Content().Size().Width - 15,
			Height: a.mainWindow.Content().Size().Height - 30,
		}
		a.settingsPopup.Resize(size)
		a.settingsPopup.Show()
	})
}

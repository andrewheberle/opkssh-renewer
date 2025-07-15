package tray

import (
	"bytes"
	"embed"
	"fmt"
	"image/png"
	"log/slog"
	"sync"
	"time"

	"github.com/andrewheberle/opkssh-renewer/internal/pkg/opkssh"
	"github.com/gen2brain/beeep"
	"github.com/getlantern/systray"
	"github.com/sergeymakinen/go-ico"
)

type Application struct {
	mu               sync.Mutex
	done             chan bool
	fs               embed.FS
	icon             []byte
	notificationIcon []byte
	title            string

	mForce  *systray.MenuItem
	mRenew  *systray.MenuItem
	mStatus *systray.MenuItem
	mQuit   *systray.MenuItem

	expiryNearNotified bool
	expiredNotified    bool

	renewer *opkssh.Renewer
}

func New(title string, fs embed.FS) (*Application, error) {
	renewer, err := opkssh.NewRenewer("id_opkssh", time.Hour*24)
	if err != nil {
		return nil, err
	}

	f, err := fs.Open("icons/app.png")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := ico.Encode(buf, img); err != nil {
		return nil, err
	}

	nbuf := new(bytes.Buffer)
	if err := png.Encode(nbuf, img); err != nil {
		return nil, err
	}

	return &Application{
		done:             make(chan bool),
		fs:               fs,
		renewer:          renewer,
		icon:             buf.Bytes(),
		notificationIcon: nbuf.Bytes(),
		title:            title,
	}, nil
}

func (app *Application) Run() {
	systray.Run(app.onReady, func() {})
}

func (app *Application) onReady() {
	systray.SetTitle(app.title)
	systray.SetIcon(app.icon)

	systray.SetTooltip(app.statusText())

	app.mRenew = systray.AddMenuItem("Renew", "Renew identity")
	if app.renewer.IdentityAge() < time.Hour*23 {
		app.mRenew.Disable()
	}
	app.mForce = systray.AddMenuItem("Force Renewal", "Force identity renewal")
	systray.AddSeparator()
	app.mStatus = systray.AddMenuItem(app.status(), app.statusText())
	app.mStatus.Disable()
	systray.AddSeparator()
	app.mQuit = systray.AddMenuItem("Quit", "Close application")

	// start background updates of ui
	go app.updateTicker()

	// handle clicks
	go app.eventloop()
}

func (app *Application) updateTicker() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()

	for {
		select {
		case <-app.done:
			return
		case <-t.C:
			app.mStatus.SetTitle(app.status())
			app.mStatus.SetTooltip(app.statusText())
			systray.SetTooltip(app.statusText())

			if app.renewer.IdentityAge() > time.Hour*24 {
				// re-enable renew menu item
				app.mRenew.Enable()

				// only send notification once
				if !app.expiredNotified {
					if err := beeep.Notify("Identity Expired", "The current identity has expired and should be renewed", app.notificationIcon); err != nil {
						slog.Error("could not send notification", "error", err)
						return
					}

					// mark as successfully notified
					app.expiredNotified = true
				}
				return
			}

			if app.renewer.IdentityAge() > time.Hour*23 {
				// re-enable renew menu item
				app.mRenew.Enable()

				// only send notification once
				if !app.expiryNearNotified {
					if err := beeep.Notify("Identity Nearly Expired", "The current identity is close to expiry and should be renewed soon", app.notificationIcon); err != nil {
						slog.Error("could not send notification", "error", err)

						return
					}

					// mark as successfully notified
					app.expiryNearNotified = true
				}
				return
			}

			// make sure renew menu item is disabled
			app.mRenew.Disable()
		}
	}
}

func (app *Application) eventloop() {
	for {
		select {
		case <-app.mRenew.ClickedCh:
			app.renew(false)
		case <-app.mForce.ClickedCh:
			app.renew(true)
		case <-app.mQuit.ClickedCh:
			app.done <- true
			systray.Quit()
			return
		}
	}
}

func (app *Application) renew(forced bool) {
	// dont do anything if not old enough at not forced
	if app.renewer.IdentityAge() < time.Hour*23 && !forced {
		return
	}

	// re-enable menu items at end
	defer func() {
		app.mRenew.Enable()
		app.mForce.Enable()
	}()

	// disable so we aren't doing two things at a time
	app.mRenew.Disable()
	app.mForce.Disable()

	// always run forced renewal as we are checking life/exipry ourselves
	if err := app.renewer.ForceRenew(); err != nil {
		if err := beeep.Notify("Error", fmt.Sprintf("Error during identity renewal: %s", err), app.notificationIcon); err != nil {
			slog.Error("could not send notification", "error", err)
		}
		return
	}

	if err := beeep.Notify("Identity Renewed", "The identity was successfully renewed", app.notificationIcon); err != nil {
		slog.Error("could not send notification", "error", err)
	}

	// reset various state on successful renew
	app.expiredNotified = false
	app.expiryNearNotified = false
	app.mRenew.Disable()
	app.mStatus.SetTitle(app.status())
	app.mStatus.SetTooltip(app.statusText())
	systray.SetTooltip(app.statusText())
}

func (app *Application) status() string {
	d := app.renewer.IdentityAge()
	if d == -1 {
		return "Identity Missing"
	}

	if d > time.Hour*24 {
		return "Identity Expired"
	}

	timeLeft := (time.Hour * 24) - d

	return fmt.Sprintf("Identity has %02dh%02dm left", int(timeLeft.Hours()), int(timeLeft.Minutes())%60)
}

func (app *Application) statusText() string {
	d := app.renewer.IdentityAge()
	if d == -1 {
		return "No identity found"
	}

	if d > time.Hour*24 {
		return "Identity has expired"
	}

	timeLeft := (time.Hour * 24) - d

	return fmt.Sprintf("Identity has %02dh%02dm until expiry", int(timeLeft.Hours()), int(timeLeft.Minutes())%60)
}

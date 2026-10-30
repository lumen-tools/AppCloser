//go:build !gui

package main

import (
	"os"
	"os/exec"
	"time"

	"fyne.io/systray"
)

// runTray shows the tray icon and monitors. It uses no GUI toolkit, so it
// stays small; the settings window is a separate process.
func runTray(openSettings bool) {
	if !instanceLock("tray", func(msg string) {
		switch msg {
		case "show":
			openSettingsWindow()
		case "quit":
			quitTray()
		} // other messages (e.g. "ping") only check that the tray runs
	}) {
		// Already running: just open the settings window there.
		sendInstance("tray", "show")
		return
	}
	loadConfig()
	// Monitoring must not depend on the tray icon (e.g. a taskbar that is not
	// ready yet at login), so it starts first.
	go runMonitor(sendNotification, reloadIfChanged, true, externalMonitorActive)
	if autostartEnabled() {
		ensureBackground()
	}
	if openSettings {
		openSettingsWindow()
	}
	systray.Run(trayReady, nil)
	// Only reached if the tray icon could not be created: keep monitoring.
	select {}
}

func trayReady() {
	paused := snapshot().Paused
	setTrayIcon(paused)
	systray.SetTooltip("AppCloser")
	mOpen := systray.AddMenuItem("Open settings", "")
	mPause := systray.AddMenuItemCheckbox("Pause monitoring", "", paused)
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit AppCloser", "")
	systray.SetOnTapped(openSettingsWindow)

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openSettingsWindow()
			case <-mPause.ClickedCh:
				updateConfig(func(c *Config) { c.Paused = !c.Paused })
			case <-mQuit.ClickedCh:
				quitTray()
			}
		}
	}()

	// Keep icon and menu in sync with changes made in the settings window.
	go func() {
		for range time.Tick(time.Second) {
			reloadIfChanged()
			if p := snapshot().Paused; p != paused {
				paused = p
				setTrayIcon(p)
				if p {
					mPause.Check()
				} else {
					mPause.Uncheck()
				}
			}
		}
	}()
}

func openSettingsWindow() {
	if sendInstance("settings", "show") {
		return
	}
	cmd := exec.Command(exePath(settingsExe))
	cmd.Env = origEnv
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}

func quitTray() {
	stopBackground()
	sendInstance("settings", "quit")
	systray.Quit()
	endAll()
	os.Exit(0)
}

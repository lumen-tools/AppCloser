//go:build !gui

package main

import (
	"fyne.io/systray"

	"appcloser/logo"
)

func setTrayIcon(paused bool) {
	p := logo.Blue
	if paused {
		p = logo.Gray
	}
	systray.SetIcon(logo.PNG(64, p))
}

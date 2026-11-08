//go:build !gui

package main

import (
	"fyne.io/systray"

	"appcloser/logo"
)

// setTrayIcon shows a template image in the menu bar, so macOS tints it to
// match the menu bar in light and dark mode. Paused: drawn at half opacity.
func setTrayIcon(paused bool) {
	a := uint8(255)
	if paused {
		a = 110
	}
	img := logo.TemplatePNG(44, a)
	systray.SetTemplateIcon(img, img)
}

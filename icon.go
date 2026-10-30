//go:build gui

package main

import (
	"fyne.io/fyne/v2"

	"appcloser/logo"
)

var (
	iconActive = fyne.NewStaticResource("appcloser.png", logo.PNG(256, logo.Blue))
	iconPaused = fyne.NewStaticResource("appcloser-paused.png", logo.PNG(256, logo.Gray))
)

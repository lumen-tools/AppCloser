//go:build !gui

package main

import (
	"os"
	"path/filepath"

	"fyne.io/systray"

	"appcloser/logo"
)

// setTrayIcon writes the icon into the program folder (the systray library
// would otherwise put a temp file into %TEMP%).
func setTrayIcon(paused bool) {
	name, p := "tray.ico", logo.Blue
	if paused {
		name, p = "tray-paused.ico", logo.Gray
	}
	path := filepath.Join(exeDir(), "AppCloser-Cache", name)
	if _, err := os.Stat(path); err != nil {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, logo.ICO(p, 16, 20, 24, 32, 48), 0o644)
	}
	systray.SetIconFromFilePath(path)
}

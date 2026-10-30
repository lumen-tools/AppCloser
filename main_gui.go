//go:build gui

package main

func main() {
	leaveProgramFolder()
	// Make sure the tray (which does the actual monitoring) is running when
	// the settings window is opened directly.
	if !instanceRunning("tray") {
		startTray()
	}
	runSettings()
}

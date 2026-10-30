//go:build !gui

package main

import "os"

// AppCloser is split into two programs so the always-running part stays small:
//
//	AppCloser            tray icon + monitoring; opens the settings window
//	AppCloser --autostart  same, without opening the settings window
//	AppCloser --headless   monitoring only (Linux background service, also Gaming Mode)
//	AppCloser-Settings   settings window (built with -tags gui, ends when closed)
func main() {
	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	leaveProgramFolder()
	switch mode {
	case "--headless":
		runHeadless()
	default:
		runTray(mode != "--autostart")
	}
}

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// origEnv is the environment before isolateEnv; child processes get this one.
var origEnv = os.Environ()

var origHome, _ = os.UserHomeDir()

// isolateEnv points every cache/config location the GUI toolkit (and the GL
// driver) would write to into a folder next to the executable, so deleting
// the program folder removes everything.
func isolateEnv() {
	data := filepath.Join(exeDir(), "AppCloser-Cache")
	os.MkdirAll(filepath.Join(data, "cache"), 0o755)
	for k, v := range isolatedVars(data) {
		os.Setenv(k, v)
	}
}

// exePath returns the path of one of AppCloser's programs in its folder.
func exePath(name string) string {
	return filepath.Join(exeDir(), name)
}

// startTray launches the tray program (without opening the settings window).
func startTray() {
	cmd := exec.Command(exePath(trayExe), "--autostart")
	cmd.Env = origEnv
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}

// leaveProgramFolder switches the working directory away from the program
// folder. On Windows a process (and every program it starts) whose working
// directory is inside a folder prevents that folder from being deleted.
func leaveProgramFolder() {
	if origHome != "" {
		os.Chdir(origHome)
	}
}

// endAll makes sure no AppCloser process is left after quitting: whatever
// still runs from the program folder after a short grace period is ended.
func endAll() {
	time.Sleep(700 * time.Millisecond)
	for _, pid := range ownProcesses() {
		killProcess(pid)
	}
}

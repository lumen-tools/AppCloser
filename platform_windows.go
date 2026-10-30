package main

import (
	"appcloser/logo"

	"os"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func listProcesses() []Proc {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var procs []Proc
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		procs = append(procs, Proc{PID: int(e.ProcessID), PPID: int(e.ParentProcessID), Name: windows.UTF16ToString(e.ExeFile[:])})
	}
	return procs
}

func killProcess(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}

func pickerEntries() []Program {
	seen := map[string]bool{}
	var out []Program
	for _, p := range listProcesses() {
		k := strings.ToLower(p.Name)
		if p.PID <= 4 || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, Program{Label: p.Name, Match: p.Name})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Match) < strings.ToLower(out[j].Match) })
	return out
}

const pickerHint = "Tip: Start the program first so it shows up here, or use “Choose file…” to select its .exe."

// --- Autostart via HKCU\...\Run ---

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

const (
	trayExe     = "AppCloser.exe"
	settingsExe = "AppCloser-Settings.exe"
)

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("AppCloser")
	return err == nil && strings.Contains(strings.ToLower(v), strings.ToLower(exePath(trayExe)))
}

func setAutostart(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		err := k.DeleteValue("AppCloser")
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	return k.SetStringValue("AppCloser", `"`+exePath(trayExe)+`" --autostart`)
}

const autostartHint = "Starts automatically when you sign in to Windows. Best untick this before deleting the folder (a leftover entry would be harmless, though)."

// --- Single instance + messages via named mutex / named events ---

var instanceMsgs = []string{"show", "quit"}

func instanceLock(instance string, onMsg func(msg string)) bool {
	mutexName, _ := windows.UTF16PtrFromString(`Local\AppCloser-` + instance)
	if _, err := windows.CreateMutex(nil, false, mutexName); err == windows.ERROR_ALREADY_EXISTS {
		return false
	}
	for _, msg := range instanceMsgs {
		name, _ := windows.UTF16PtrFromString(`Local\AppCloser-` + instance + "-" + msg)
		ev, err := windows.CreateEvent(nil, 0, 0, name)
		if err != nil {
			continue
		}
		go func(msg string) {
			for {
				if s, _ := windows.WaitForSingleObject(ev, windows.INFINITE); s == windows.WAIT_OBJECT_0 {
					onMsg(msg)
				}
			}
		}(msg)
	}
	return true
}

func sendInstance(instance, msg string) bool {
	name, _ := windows.UTF16PtrFromString(`Local\AppCloser-` + instance + "-" + msg)
	ev, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(ev)
	return windows.SetEvent(ev) == nil
}

func externalMonitorActive() bool { return false }

// --- Notifications: shown by AppCloser's own tray icon (no helper process) ---

// notifyIconData mirrors NOTIFYICONDATAW (976 bytes on amd64).
type notifyIconData struct {
	Size                       uint32
	Wnd                        windows.Handle
	ID, Flags, CallbackMessage uint32
	Icon                       windows.Handle
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Timeout                    uint32 // union with uVersion
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GuidItem                   windows.GUID
	BalloonIcon                windows.Handle
}

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	pFindWindowExW      = user32.NewProc("FindWindowExW")
	pGetWindowThreadPID = user32.NewProc("GetWindowThreadProcessId")
	pShellNotifyIconW   = shell32.NewProc("Shell_NotifyIconW")
	pCreateIconFromRes  = user32.NewProc("CreateIconFromResourceEx")
	balloonIcon         windows.Handle
	trayClassName, _    = windows.UTF16PtrFromString("SystrayClass")
)

// trayWindow finds the hidden window that owns our tray icon (created by the
// systray library with class "SystrayClass" and icon ID 100).
func trayWindow() windows.Handle {
	self := uint32(os.Getpid())
	var h uintptr
	for {
		h, _, _ = pFindWindowExW.Call(0, h, uintptr(unsafe.Pointer(trayClassName)), 0)
		if h == 0 {
			return 0
		}
		var pid uint32
		pGetWindowThreadPID.Call(h, uintptr(unsafe.Pointer(&pid)))
		if pid == self {
			return windows.Handle(h)
		}
	}
}

func sendNotification(title, body string) {
	const (
		nimModify = 0x00000001
		nifInfo   = 0x00000010
		niifUser  = 0x00000004 // custom icon in BalloonIcon
		niifLarge = 0x00000020
	)
	if balloonIcon == 0 {
		// CreateIconFromResourceEx accepts PNG data directly.
		png := logo.PNG(64, logo.Blue)
		h, _, _ := pCreateIconFromRes.Call(uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)), 1, 0x00030000, 64, 64, 0)
		balloonIcon = windows.Handle(h)
	}
	wnd := trayWindow()
	if wnd == 0 {
		return
	}
	nid := notifyIconData{Wnd: wnd, ID: 100, Flags: nifInfo, InfoFlags: niifUser | niifLarge, BalloonIcon: balloonIcon}
	nid.Size = uint32(unsafe.Sizeof(nid))
	copy(nid.InfoTitle[:len(nid.InfoTitle)-1], windows.StringToUTF16(title))
	copy(nid.Info[:len(nid.Info)-1], windows.StringToUTF16(body))
	pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func isolatedVars(data string) map[string]string {
	return map[string]string{
		"USERPROFILE":  data,
		"LOCALAPPDATA": filepath.Join(data, "Local"),
		"APPDATA":      filepath.Join(data, "AppData", "Roaming"),
	}
}

func runHeadless() {
	if !instanceLock("headless", func(string) {}) {
		return
	}
	runMonitor(func(string, string) {}, reloadIfChanged, true, nil)
}

func ensureBackground() {}
func stopBackground()   {}

// procRunning: exited processes never appear in the snapshot on Windows.
func procRunning(int) bool { return true }

func instanceRunning(instance string) bool {
	name, _ := windows.UTF16PtrFromString(`Local\AppCloser-` + instance + "-show")
	ev, err := windows.OpenEvent(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return false
	}
	windows.CloseHandle(ev)
	return true
}

// procCreated returns the creation time of a process (FILETIME), 0 if unknown.
func procCreated(pid int) int64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	var created, exit, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exit, &kernel, &user) != nil {
		return 0
	}
	return created.Nanoseconds()
}

// ownProcesses returns the IDs of other running processes whose program file
// lies in AppCloser's folder.
func ownProcesses() []int {
	dir := strings.ToLower(exeDir()) + `\`
	var pids []int
	for _, p := range listProcesses() {
		if p.PID == os.Getpid() || p.PID <= 4 {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(p.PID))
		if err != nil {
			continue
		}
		buf := make([]uint16, windows.MAX_LONG_PATH)
		n := uint32(len(buf))
		if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) == nil &&
			strings.HasPrefix(strings.ToLower(windows.UTF16ToString(buf[:n])), dir) {
			pids = append(pids, p.PID)
		}
		windows.CloseHandle(h)
	}
	return pids
}

// procIsSystem reports whether a process runs from the Windows folder.
func procIsSystem(pid int) bool {
	winDir, err := windows.GetWindowsDirectory()
	if err != nil || winDir == "" {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) != nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(windows.UTF16ToString(buf[:n])), strings.ToLower(winDir)+`\`)
}

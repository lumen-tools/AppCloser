package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// --- Processes (sysctl) ---

type procInfo struct {
	seen     int // scan number in which the process was last listed
	start    int64
	path     string // full path of the executable
	name     string
	firstSee time.Time
}

var (
	procCache   = map[int]*procInfo{}
	procCacheMu sync.Mutex
	scanCount   int
)

func startOf(k *unix.KinfoProc) int64 {
	return k.Proc.P_starttime.Sec*1_000_000 + int64(k.Proc.P_starttime.Usec)
}

// execPath reads the executable path of a process (kern.procargs2), which,
// unlike the process name, is not cut off after 16 characters.
func execPath(pid int) string {
	b, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil || len(b) < 5 {
		return ""
	}
	b = b[4:] // argc
	if i := bytes.IndexByte(b, 0); i > 0 {
		return string(b[:i])
	}
	return ""
}

func listProcesses() []Proc {
	procCacheMu.Lock()
	defer procCacheMu.Unlock()
	kps, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil
	}
	scanCount++
	refreshAll := scanCount%50 == 0
	uid := uint32(os.Getuid())
	now := time.Now()
	procs := make([]Proc, 0, 256)
	for i := range kps {
		k := &kps[i]
		pid := int(k.Proc.P_pid)
		if pid <= 0 || k.Eproc.Ucred.Uid != uid {
			continue
		}
		info := procCache[pid]
		if info == nil || info.start != startOf(k) {
			info = &procInfo{start: startOf(k), firstSee: now}
			procCache[pid] = info
			refreshName(info, pid, k)
		} else if refreshAll || now.Sub(info.firstSee) < 3*time.Second {
			refreshName(info, pid, k) // exec() changes the program, not the pid
		}
		info.seen = scanCount
		procs = append(procs, Proc{PID: pid, PPID: int(k.Eproc.Ppid), Name: info.name})
	}
	for pid, info := range procCache {
		if info.seen != scanCount {
			delete(procCache, pid)
		}
	}
	return procs
}

func refreshName(info *procInfo, pid int, k *unix.KinfoProc) {
	if p := execPath(pid); p != "" {
		info.path, info.name = p, filepath.Base(p)
		return
	}
	info.name = unix.ByteSliceToString(k.Proc.P_comm[:])
}

func cachedPath(pid int) string {
	procCacheMu.Lock()
	defer procCacheMu.Unlock()
	if info := procCache[pid]; info != nil && info.path != "" {
		return info.path
	}
	return execPath(pid)
}

func killProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}

// procCreated returns the start time of a process in microseconds, 0 if unknown.
func procCreated(pid int) int64 {
	k, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || k.Proc.P_pid != int32(pid) {
		return 0
	}
	return startOf(k)
}

// procRunning reports false for zombie processes (exited, not yet reaped).
func procRunning(pid int) bool {
	k, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	return err == nil && k.Proc.P_pid == int32(pid) && k.Proc.P_stat != 5 // SZOMB
}

// procIsSystem reports whether a process is part of macOS itself.
func procIsSystem(pid int) bool {
	p := cachedPath(pid)
	for _, prefix := range []string{"/System/", "/usr/libexec/", "/usr/sbin/", "/sbin/", "/Library/Apple/"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// ownProcesses returns the IDs of other running processes of this AppCloser.
func ownProcesses() []int {
	dir := exeDir() + "/"
	var pids []int
	for _, p := range listProcesses() {
		if p.PID != os.Getpid() && strings.HasPrefix(cachedPath(p.PID), dir) {
			pids = append(pids, p.PID)
		}
	}
	return pids
}

// --- Program picker: installed apps first, then running programs ---

var plistExecutable = regexp.MustCompile(`<key>CFBundleExecutable</key>\s*<string>([^<]+)</string>`)

func pickerEntries() []Program {
	seen := map[string]bool{}
	var out []Program
	add := func(p Program) {
		if k := strings.ToLower(p.Match); p.Match != "" && !seen[k] {
			seen[k] = true
			out = append(out, p)
		}
	}
	home, _ := os.UserHomeDir()
	var apps []Program
	for _, dir := range []string{"/Applications", "/Applications/Utilities", filepath.Join(home, "Applications")} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".app")
			exe := name
			if b, err := os.ReadFile(filepath.Join(dir, e.Name(), "Contents", "Info.plist")); err == nil {
				if m := plistExecutable.FindSubmatch(b); m != nil {
					exe = string(m[1])
				}
			}
			apps = append(apps, Program{Label: name, Match: exe})
		}
	}
	sort.Slice(apps, func(i, j int) bool { return strings.ToLower(apps[i].Label) < strings.ToLower(apps[j].Label) })
	for _, a := range apps {
		add(a)
	}
	var running []Program
	for _, p := range listProcesses() {
		running = append(running, Program{Label: p.Name, Match: p.Name})
	}
	sort.Slice(running, func(i, j int) bool { return strings.ToLower(running[i].Match) < strings.ToLower(running[j].Match) })
	for _, p := range running {
		add(p)
	}
	return out
}

const pickerHint = "Apps from the Applications folder are listed first, then running programs."

// --- Autostart: a LaunchAgent that starts the tray at login ---

const (
	trayExe     = "AppCloser"
	settingsExe = "AppCloser-Settings"
	agentLabel  = "de.appcloser.tray"
)

func agentFile() string {
	return filepath.Join(origHome, "Library", "LaunchAgents", agentLabel+".plist")
}

func autostartEnabled() bool {
	b, err := os.ReadFile(agentFile())
	return err == nil && strings.Contains(string(b), exePath(trayExe))
}

func setAutostart(on bool) error {
	if !on {
		if err := os.Remove(agentFile()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--autostart</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, agentLabel, xmlEscape(exePath(trayExe)))
	if err := os.MkdirAll(filepath.Dir(agentFile()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(agentFile(), []byte(plist), 0o644)
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

const autostartHint = "Starts when you log in. If you delete AppCloser, the login item does nothing."

func ensureBackground() {}
func stopBackground()   {}

func externalMonitorActive() bool { return false }

func runHeadless() {
	if !instanceLock("headless", func(string) {}) {
		return
	}
	runMonitor(func(string, string) {}, reloadIfChanged, true, nil)
}

// --- Single instance + messages via unix sockets in the per-user temp folder ---

func socketPath(instance string) string {
	return filepath.Join(os.TempDir(), "AppCloser-"+instance+".sock")
}

func instanceLock(instance string, onMsg func(msg string)) bool {
	path := socketPath(instance)
	l, err := net.Listen("unix", path)
	if err != nil {
		if instanceRunning(instance) {
			return false
		}
		os.Remove(path) // left over from a process that was killed
		if l, err = net.Listen("unix", path); err != nil {
			return false
		}
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			b, _ := io.ReadAll(io.LimitReader(c, 64))
			c.Close()
			onMsg(string(b))
		}
	}()
	return true
}

func sendInstance(instance, msg string) bool {
	c, err := net.Dial("unix", socketPath(instance))
	if err != nil {
		return false
	}
	c.Write([]byte(msg))
	c.Close()
	return true
}

func instanceRunning(instance string) bool {
	return sendInstance(instance, "ping")
}

// The settings window keeps its toolkit caches in the program folder.
func isolatedVars(data string) map[string]string {
	return map[string]string{"HOME": data}
}

package main

import (
	"appcloser/logo"
	"io"

	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

var flatpakScope = regexp.MustCompile(`app-flatpak-(.+)-\d+\.scope`)

type procInfo struct {
	seen     int  // scan number in which the process was last listed
	own      bool // belongs to the current user; other users' processes are ignored
	ppid     int
	name     string
	flatpak  string
	firstSee time.Time
}

var (
	procCache   = map[int]*procInfo{}
	procCacheMu sync.Mutex
	scanCount   int
)

// listProcesses keeps a cache so that a typical poll only lists /proc and
// does no per-process work. New processes are inspected once; for their first
// seconds they are re-checked (exec() changes the name, the flatpak scope is
// assigned shortly after launch). All names are refreshed every 10 seconds.
func listProcesses() []Proc {
	procCacheMu.Lock()
	defer procCacheMu.Unlock()
	d, err := os.Open("/proc")
	if err != nil {
		return nil
	}
	names, _ := d.Readdirnames(-1)
	d.Close()
	scanCount++
	refreshAll := scanCount%50 == 0
	uid := os.Getuid()
	now := time.Now()
	procs := make([]Proc, 0, 256)
	for _, n := range names {
		if n[0] < '0' || n[0] > '9' {
			continue
		}
		pid, err := strconv.Atoi(n)
		if err != nil {
			continue
		}
		dir := "/proc/" + n
		info := procCache[pid]
		if info != nil {
			info.seen = scanCount
		}
		if info == nil {
			info = &procInfo{firstSee: now, seen: scanCount}
			st, err := os.Stat(dir)
			if err != nil {
				continue
			}
			info.own = int(st.Sys().(*syscall.Stat_t).Uid) == uid
			if info.own {
				stat, err := os.ReadFile(dir + "/stat")
				if err != nil {
					continue
				}
				info.ppid = statPPID(stat)
				info.name = procName(dir)
				info.flatpak = flatpakOf(dir)
			}
			procCache[pid] = info
		} else if info.own {
			young := now.Sub(info.firstSee) < 3*time.Second
			if young || refreshAll {
				if name := procName(dir); name != "" {
					info.name = name
				}
			}
			if young && info.flatpak == "" {
				info.flatpak = flatpakOf(dir)
			}
		}
		if info.own && info.name != "" {
			procs = append(procs, Proc{PID: pid, PPID: info.ppid, Name: info.name, Flatpak: info.flatpak})
		}
	}
	for pid, info := range procCache {
		if info.seen != scanCount {
			delete(procCache, pid)
		}
	}
	return procs
}

// statPPID returns the parent pid (field 4) of /proc/<pid>/stat.
func statPPID(stat []byte) int {
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 {
		if f := strings.Fields(s[i+1:]); len(f) > 1 {
			ppid, _ := strconv.Atoi(f[1])
			return ppid
		}
	}
	return 0
}

// procName returns the executable name of a process.
func procName(dir string) string {
	if exe, err := os.Readlink(dir + "/exe"); err == nil {
		return filepath.Base(strings.TrimSuffix(exe, " (deleted)"))
	}
	if comm, err := os.ReadFile(dir + "/comm"); err == nil {
		return strings.TrimSpace(string(comm))
	}
	return ""
}

func flatpakOf(dir string) string {
	if cg, err := os.ReadFile(dir + "/cgroup"); err == nil {
		if m := flatpakScope.FindSubmatch(cg); m != nil {
			return string(m[1])
		}
	}
	return flatpakInfo(dir)
}

func flatpakInfo(dir string) string {
	f, err := os.Open(dir + "/root/.flatpak-info")
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if v, ok := strings.CutPrefix(s.Text(), "name="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func killProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}

// pickerEntries lists running processes plus installed flatpak apps.
func pickerEntries() []Program {
	seen := map[string]bool{}
	var out []Program
	add := func(p Program) {
		if !seen[p.Match] {
			seen[p.Match] = true
			out = append(out, p)
		}
	}
	cmd := exec.Command("flatpak", "list", "--app", "--columns=application,name")
	cmd.Env = origEnv
	if b, err := cmd.Output(); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.SplitN(line, "\t", 2)
			if len(f) == 2 && f[0] != "" {
				add(Program{Label: strings.TrimSpace(f[1]), Match: "flatpak:" + strings.TrimSpace(f[0])})
			}
		}
	}
	var running []Program
	for _, p := range listProcesses() {
		if p.Flatpak != "" {
			add(Program{Label: p.Flatpak, Match: "flatpak:" + p.Flatpak})
		}
		running = append(running, Program{Label: p.Name, Match: p.Name})
	}
	sort.Slice(running, func(i, j int) bool { return strings.ToLower(running[i].Match) < strings.ToLower(running[j].Match) })
	for _, p := range running {
		add(p)
	}
	return out
}

const pickerHint = "Flatpak apps (e.g. installed via Discover) are listed first and are always detected completely, including all of their subprocesses."

// --- Autostart ---
// Two parts: an XDG autostart entry for the tray icon in desktop mode, and a
// systemd user service running "--headless", which also covers Gaming Mode.

func userConfigDir() string {
	if origConfigHome != "" {
		return origConfigHome
	}
	return filepath.Join(origHome, ".config")
}

func autostartFile() string {
	return filepath.Join(userConfigDir(), "autostart", "AppCloser.desktop")
}

func serviceFile() string {
	return filepath.Join(userConfigDir(), "systemd", "user", serviceName)
}

const serviceName = "appcloser.service"

const (
	trayExe     = "AppCloser"
	settingsExe = "AppCloser-Settings"
)

func autostartEnabled() bool {
	b, err := os.ReadFile(autostartFile())
	return err == nil && strings.Contains(string(b), exePath(trayExe))
}

func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	cmd.Env = origEnv
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func setAutostart(on bool) error {
	if !on {
		systemctl("disable", "--now", serviceName)
		for _, f := range []string{autostartFile(), serviceFile()} {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		systemctl("daemon-reload")
		return nil
	}
	exe := exePath(trayExe)
	// TryExec / ConditionPathExists make the system ignore both entries once
	// the program folder is deleted.
	desktop := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=AppCloser\nComment=Closes programs when others start\nTryExec=%s\nExec=\"%s\" --autostart\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", exe, exe)
	service := fmt.Sprintf("[Unit]\nDescription=AppCloser background monitoring\nConditionPathExists=%s\n\n[Service]\nExecStart=\"%s\" --headless\nRestart=on-failure\nRestartSec=3\n\n[Install]\nWantedBy=default.target\n", exe, exe)
	for f, content := range map[string]string{autostartFile(): desktop, serviceFile(): service} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
			return err
		}
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	return systemctl("enable", "--now", serviceName)
}

const autostartHint = "Monitors from power-on – also in Gaming Mode (invisible). In Desktop Mode the tray icon appears as well. “Quit” stops everything until the next restart. If the folder is deleted, the system ignores the autostart entries."

// --- Single instance + messages via abstract unix sockets (leave no files) ---

func socketName(instance string) string {
	return "@AppCloser-" + instance + "-" + strconv.Itoa(os.Getuid())
}

// instanceLock makes this process the only one of its kind ("tray",
// "settings", "headless"). Messages sent with sendInstance arrive at onMsg.
func instanceLock(instance string, onMsg func(msg string)) bool {
	l, err := net.Listen("unix", socketName(instance))
	if err != nil {
		return false
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
	c, err := net.Dial("unix", socketName(instance))
	if err != nil {
		return false
	}
	c.Write([]byte(msg))
	c.Close()
	return true
}

var (
	extMu      sync.Mutex
	extChecked time.Time
	extActive  bool
)

// externalMonitorActive reports whether the background service is running;
// the tray then leaves monitoring (and notifications) to it.
func externalMonitorActive() bool {
	extMu.Lock()
	defer extMu.Unlock()
	if time.Since(extChecked) > 5*time.Second {
		extChecked = time.Now()
		c, err := net.Dial("unix", socketName("headless"))
		extActive = err == nil
		if c != nil {
			c.Close()
		}
	}
	return extActive
}

// --- Notifications via org.freedesktop.Notifications ---

var (
	iconOnce sync.Once
	iconPath string
)

// notifyIconPath writes the AppCloser logo into the program folder once, so
// notifications show AppCloser's own icon.
func notifyIconPath() string {
	iconOnce.Do(func() {
		p := filepath.Join(exeDir(), "AppCloser-Cache", "appcloser.png")
		os.MkdirAll(filepath.Dir(p), 0o755)
		if os.WriteFile(p, logo.PNG(128, logo.Blue), 0o644) == nil {
			iconPath = p
		}
	})
	return iconPath
}

func sendNotification(title, body string) {
	go func() {
		conn, err := dbus.SessionBus()
		if err != nil {
			return
		}
		obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
		// The body may be interpreted as markup; escape it so names stay visible.
		body := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(body)
		// NoAutoStart: never launch a notification daemon (e.g. in Gaming Mode).
		obj.Call("org.freedesktop.Notifications.Notify", dbus.FlagNoAutoStart, "AppCloser", uint32(0),
			notifyIconPath(), title, body, []string{}, map[string]dbus.Variant{}, int32(5000))
	}()
}

var origConfigHome = os.Getenv("XDG_CONFIG_HOME")

func isolatedVars(data string) map[string]string {
	return map[string]string{
		"XDG_CACHE_HOME":            filepath.Join(data, "cache"),
		"XDG_CONFIG_HOME":           filepath.Join(data, "config"),
		"MESA_SHADER_CACHE_DISABLE": "true",
	}
}

// runHeadless monitors without any GUI (systemd service, works in Gaming Mode).
// In Desktop Mode its notifications appear like the tray's; in Gaming Mode
// there is no notification daemon and they are silently dropped.
func runHeadless() {
	if !instanceLock("headless", func(string) {}) {
		return
	}
	runMonitor(sendNotification, reloadIfChanged, true, nil)
}

// ensureBackground (re)starts the background service, e.g. after AppCloser
// was quit earlier in this session.
func ensureBackground() {
	go systemctl("start", serviceName)
}

// stopBackground stops the service for this session; it starts again at the
// next boot when autostart is enabled.
func stopBackground() {
	systemctl("stop", serviceName)
}

// procRunning reports false for zombie processes (exited, not yet reaped).
func procRunning(pid int) bool {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	s := string(stat)
	i := strings.LastIndexByte(s, ')')
	return i < 0 || i+2 >= len(s) || s[i+2] != 'Z'
}

func instanceRunning(instance string) bool {
	return sendInstance(instance, "ping")
}

// procCreated returns the start time of a process (clock ticks since boot),
// 0 if unknown.
func procCreated(pid int) int64 {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0
	}
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 {
		if f := strings.Fields(s[i+1:]); len(f) > 19 {
			t, _ := strconv.ParseInt(f[19], 10, 64)
			return t
		}
	}
	return 0
}

// ownProcesses returns the IDs of other running processes whose program file
// lies in AppCloser's folder.
func ownProcesses() []int {
	dir := exeDir() + "/"
	var pids []int
	for _, p := range listProcesses() {
		if p.PID == os.Getpid() {
			continue
		}
		if exe, err := os.Readlink("/proc/" + strconv.Itoa(p.PID) + "/exe"); err == nil && strings.HasPrefix(exe, dir) {
			pids = append(pids, p.PID)
		}
	}
	return pids
}

// procIsSystem: on Linux, programs started by a trigger are never shared
// system helpers of this kind.
func procIsSystem(int) bool { return false }

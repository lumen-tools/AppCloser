package main

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Proc is a running process as seen by the monitor.
type Proc struct {
	PID     int
	PPID    int
	Name    string // executable file name
	Flatpak string // flatpak app id, empty if not sandboxed
}

const (
	pollInterval = 200 * time.Millisecond
	// How long a finished trigger process still passes its role on to
	// children it started (launchers that start the real program and exit).
	inheritGrace = 10 * time.Second
	// A trigger that exits within this time after it was seen counts as a
	// launcher: the programs it started stay triggers. Leftovers of a trigger
	// that ran longer (e.g. a helper the closed program left behind) do not.
	launcherWindow = 30 * time.Second
)

func normName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.TrimSuffix(s, ".exe")
}

func (p Proc) matches(prog Program) bool {
	m := strings.TrimSpace(prog.Match)
	if id, ok := strings.CutPrefix(m, "flatpak:"); ok {
		return p.Flatpak != "" && strings.EqualFold(p.Flatpak, id)
	}
	return m != "" && normName(p.Name) == normName(m)
}

func anyMatch(p Proc, progs []Program) bool {
	for _, prog := range progs {
		if p.matches(prog) {
			return true
		}
	}
	return false
}

func progName(p Program) string {
	if l := strings.TrimSpace(p.Label); l != "" {
		return l
	}
	if id, ok := strings.CutPrefix(strings.TrimSpace(p.Match), "flatpak:"); ok {
		return id
	}
	return strings.TrimSpace(p.Match)
}

// triggerProc is a process that counts as a running trigger, either directly
// or because a trigger process started it.
type triggerProc struct {
	desc     string // e.g. "BatGui.exe" or "BatGui-Core.exe (started by BatGui.exe)"
	origin   string // name of the configured trigger it descends from
	name     string // process name, guards against reused process IDs
	created  int64  // process creation time (platform units, 0 = unknown)
	lastSeen time.Time
	// The directly matched trigger process this one descends from (itself for
	// a direct match), and when it was first and last seen running.
	root                int
	rootFirst, rootLast time.Time
}

func (t triggerProc) inherited(pid int) bool { return t.root != pid }

// rootWasLauncher reports whether the root trigger ended soon after it was
// seen, i.e. it only started the real program.
func (t triggerProc) rootWasLauncher() bool {
	return t.rootLast.Sub(t.rootFirst) < launcherWindow
}

// createdCache remembers creation times; it is only used by the monitor goroutine.
var createdCache = map[int]struct {
	name string
	t    int64
}{}

func createdAt(p Proc) int64 {
	if c, ok := createdCache[p.PID]; ok && c.name == p.Name {
		return c.t
	}
	t := procCreated(p.PID)
	createdCache[p.PID] = struct {
		name string
		t    int64
	}{p.Name, t}
	return t
}

// systemCache remembers which processes are part of the operating system.
var systemCache = map[int]struct {
	name string
	sys  bool
}{}

func isSystemProcess(p Proc) bool {
	if c, ok := systemCache[p.PID]; ok && c.name == p.Name {
		return c.sys
	}
	sys := procIsSystem(p.PID)
	systemCache[p.PID] = struct {
		name string
		sys  bool
	}{p.Name, sys}
	return sys
}

// firstSeen remembers when the monitor first saw each process.
var firstSeen = map[int]struct {
	name string
	t    time.Time
}{}

func firstSeenAt(p Proc, now time.Time) time.Time {
	if f, ok := firstSeen[p.PID]; ok && f.name == p.Name {
		return f.t
	}
	firstSeen[p.PID] = struct {
		name string
		t    time.Time
	}{p.Name, now}
	return now
}

func pruneCreatedCache(alive map[int]bool) {
	for pid := range createdCache {
		if !alive[pid] {
			delete(createdCache, pid)
		}
	}
	for pid := range firstSeen {
		if !alive[pid] {
			delete(firstSeen, pid)
		}
	}
	for pid := range systemCache {
		if !alive[pid] {
			delete(systemCache, pid)
		}
	}
}

// RuleState is what the GUI shows per rule.
type RuleState struct {
	Trigger string // description of a running trigger, empty if none
}

var (
	stateMu    sync.Mutex
	ruleStates []RuleState
)

func currentStates() []RuleState {
	stateMu.Lock()
	defer stateMu.Unlock()
	return append([]RuleState(nil), ruleStates...)
}

// activeTriggers returns all processes that currently count as triggers for r.
// prev holds the result of the previous poll so that children of a launcher
// that already exited are still recognised.
func activeTriggers(r Rule, procs []Proc, prev map[int]triggerProc, self int, now time.Time) map[int]triggerProc {
	active := map[int]triggerProc{}
	for _, p := range procs {
		if p.PID == self || p.matches(r.Target) {
			continue
		}
		for _, t := range r.Triggers {
			if p.matches(t) {
				// Use the configured name: the matching process itself may be a
				// helper such as "bash" inside a flatpak.
				active[p.PID] = triggerProc{desc: progName(t), origin: progName(t), name: p.Name, created: createdAt(p), lastSeen: now,
					root: p.PID, rootFirst: firstSeenAt(p, now), rootLast: now}
				break
			}
		}
	}
	running := make(map[int]Proc, len(procs))
	for _, p := range procs {
		running[p.PID] = p
	}
	for _, p := range procs {
		firstSeenAt(p, now)
	}
	// lookup returns the trigger that child could descend from via its parent ID.
	lookup := func(parentPID int, child Proc) (triggerProc, bool) {
		if t, ok := active[parentPID]; ok {
			return t, true
		}
		t, ok := prev[parentPID]
		if !ok || now.Sub(t.lastSeen) >= inheritGrace {
			return t, false
		}
		// The trigger has exited. If its process ID is already used by another
		// process, that one is unrelated ...
		if p, alive := running[parentPID]; alive && (p.Name != t.name || createdAt(p) != t.created) {
			return t, false
		}
		// ... and so is any process that only appeared after the trigger was
		// gone (its parent ID may belong to a short-lived process that reused it).
		if firstSeenAt(child, now).After(t.lastSeen.Add(2 * pollInterval)) {
			return t, false
		}
		// A program that ran for a while and was closed passes nothing on.
		if _, rootAlive := active[t.root]; !rootAlive && !t.rootWasLauncher() {
			return t, false
		}
		return t, true
	}
	// A started program stays a trigger while it lives: while its root trigger
	// runs, or for good if the root was only a launcher.
	for _, p := range procs {
		t, ok := prev[p.PID]
		if !ok || t.name != p.Name || !t.inherited(p.PID) || p.matches(r.Target) || p.PID == self {
			continue
		}
		if _, done := active[p.PID]; done {
			continue
		}
		if root, alive := active[t.root]; alive && !root.inherited(t.root) {
			t.rootLast = now
		} else if !t.rootWasLauncher() {
			continue
		}
		t.lastSeen = now
		active[p.PID] = t
	}
	// Children (and grandchildren) of trigger processes are triggers too.
	for changed := true; changed; {
		changed = false
		for _, p := range procs {
			if _, done := active[p.PID]; done || p.PID == self || p.PPID == 0 || p.PPID == p.PID || p.matches(r.Target) {
				continue
			}
			if parent, ok := lookup(p.PPID, p); ok {
				// Windows keeps the parent ID of exited parents, and IDs are
				// reused: a process started before the "parent" cannot be its child.
				created := createdAt(p)
				if created != 0 && parent.created != 0 && created < parent.created {
					continue
				}
				// Windows' own helpers (e.g. splwow64.exe, the printer driver host
				// a 32-bit program starts) outlive the program and are shared.
				if isSystemProcess(p) {
					continue
				}
				name := strings.TrimSpace(p.Name)
				if name == "" {
					name = "process " + strconv.Itoa(p.PID)
				}
				active[p.PID] = triggerProc{desc: name + " (started by " + parent.origin + ")", origin: parent.origin, name: p.Name, created: created, lastSeen: now,
					root: parent.root, rootFirst: parent.rootFirst, rootLast: parent.rootLast}
				changed = true
			}
		}
	}
	// Keep recently finished trigger processes for a while (see inheritGrace),
	// so programs they started can still be recognised.
	for pid, t := range prev {
		if _, alive := running[pid]; alive {
			continue
		}
		if _, ok := active[pid]; !ok && now.Sub(t.lastSeen) < inheritGrace {
			active[pid] = t
		}
	}
	return active
}

// runMonitor polls the process list and kills targets while a trigger runs.
// With enforce=false it only computes the rule states (settings window).
// While idle() returns true the loop does no work (tray on Linux while the
// background service is running).
func runMonitor(notify func(title, body string), reload func(), enforce bool, idle func() bool) {
	self := os.Getpid()
	lastNote := map[int]time.Time{}
	var trig []map[int]triggerProc
	var c Config
	version, trigVersion := -1, -1
	loops := 0
	for {
		if idle != nil && idle() {
			time.Sleep(time.Second)
			continue
		}
		// Check the settings file about once per second.
		if loops++; reload != nil && loops%5 == 1 {
			reload()
		}
		if v := configVersion(); v != version || version < 0 {
			version, c = v, snapshot()
		}
		states := make([]RuleState, len(c.Rules))
		// Start over whenever the rules change, so removed triggers are forgotten.
		if version != trigVersion {
			trigVersion = version
			trig = make([]map[int]triggerProc, len(c.Rules))
		}
		if !c.Paused && len(c.Rules) > 0 {
			procs := listProcesses()
			alive := make(map[int]bool, len(procs))
			for _, p := range procs {
				alive[p.PID] = true
			}
			now := time.Now()
			pruneCreatedCache(alive)
			for i, r := range c.Rules {
				if !r.Enabled || strings.TrimSpace(r.Target.Match) == "" || len(r.Triggers) == 0 {
					trig[i] = nil
					continue
				}
				trig[i] = activeTriggers(r, procs, trig[i], self, now)
				// Prefer a direct match (desc == origin) over inherited ones.
				var trigger, origin, via string
				for pid, t := range trig[i] {
					if alive[pid] && procRunning(pid) && (trigger == "" || (t.desc == t.origin && trigger != origin)) {
						trigger, origin, via = t.desc, t.origin, ""
						if t.desc != t.origin {
							via = t.name
						}
					}
				}
				states[i].Trigger = trigger
				if trigger == "" || !enforce {
					continue
				}
				killed := 0
				for _, p := range procs {
					if p.PID == self || !p.matches(r.Target) || anyMatch(p, r.Triggers) {
						continue
					}
					if killProcess(p.PID) == nil {
						killed++
					}
				}
				if killed > 0 && c.Notifications && time.Since(lastNote[i]) > 5*time.Second {
					lastNote[i] = time.Now()
					body := "Because " + origin + " is running."
					if via != "" {
						body = "Because " + origin + " is running (process " + via + ")."
					}
					notify(progName(r.Target)+" closed", body)
				}
			}
		}
		stateMu.Lock()
		ruleStates = states
		stateMu.Unlock()
		time.Sleep(pollInterval)
	}
}

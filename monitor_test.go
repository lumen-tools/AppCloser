package main

import (
	"testing"
	"time"
)

func resetCaches() {
	createdCache = map[int]struct {
		name string
		t    int64
	}{}
	firstSeen = map[int]struct {
		name string
		t    time.Time
	}{}
	systemCache = map[int]struct {
		name string
		sys  bool
	}{}
}

func setCreated(pid int, name string, t int64) {
	createdCache[pid] = struct {
		name string
		t    int64
	}{name, t}
}

var batRule = Rule{Enabled: true, Target: Program{Match: "nextcloud.exe"}, Triggers: []Program{{Label: "BatGui", Match: "batgui.exe"}}}

// An old background process whose parent ID points to a long-gone process
// must not become a trigger when BatGui later gets that reused ID.
func TestReusedParentIDIsIgnored(t *testing.T) {
	resetCaches()
	setCreated(500, "helper.exe", 100) // started long ago by some old PID 1234
	setCreated(1234, "batgui.exe", 900)
	setCreated(1300, "batguicore.exe", 950) // real child of BatGui
	procs := []Proc{
		{PID: 500, PPID: 1234, Name: "helper.exe"},
		{PID: 1234, PPID: 4, Name: "batgui.exe"},
		{PID: 1300, PPID: 1234, Name: "batguicore.exe"},
	}
	now := time.Now()
	act := activeTriggers(batRule, procs, nil, 1, now)
	if _, ok := act[500]; ok {
		t.Fatal("helper.exe (older than BatGui) was treated as BatGui's child")
	}
	if _, ok := act[1300]; !ok {
		t.Fatal("real child batguicore.exe not detected")
	}
	// BatGui and its child exit: nothing may remain active.
	later := now.Add(time.Second)
	act = activeTriggers(batRule, []Proc{{PID: 500, PPID: 1234, Name: "helper.exe"}}, act, 1, later)
	for pid, tp := range act {
		if pid == 500 {
			t.Fatalf("helper.exe active after BatGui exited: %+v", tp)
		}
	}
}

// After BatGui exited, a new unrelated process reusing its ID must not pass
// the trigger role on to its children.
func TestReusedTriggerIDDuringGrace(t *testing.T) {
	resetCaches()
	setCreated(1234, "batgui.exe", 900)
	now := time.Now()
	act := activeTriggers(batRule, []Proc{{PID: 1234, PPID: 4, Name: "batgui.exe"}}, nil, 1, now)
	setCreated(1234, "notepad.exe", 2000) // ID reused
	setCreated(1400, "other.exe", 2100)
	procs := []Proc{{PID: 1234, PPID: 4, Name: "notepad.exe"}, {PID: 1400, PPID: 1234, Name: "other.exe"}}
	act = activeTriggers(batRule, procs, act, 1, now.Add(time.Second))
	if _, ok := act[1400]; ok {
		t.Fatal("child of reused ID treated as trigger")
	}
	if _, ok := act[1234]; ok && act[1234].name == "notepad.exe" {
		t.Fatal("reused ID treated as trigger")
	}
}

// A launcher that exits still passes the role to the program it started.
func TestLauncherChildStaysTrigger(t *testing.T) {
	resetCaches()
	setCreated(1234, "batgui.exe", 900)
	setCreated(1300, "batguicore.exe", 950)
	now := time.Now()
	act := activeTriggers(batRule, []Proc{{PID: 1234, PPID: 4, Name: "batgui.exe"}, {PID: 1300, PPID: 1234, Name: "batguicore.exe"}}, nil, 1, now)
	for i := 1; i <= 30; i++ { // 30 s after the launcher exited
		act = activeTriggers(batRule, []Proc{{PID: 1300, PPID: 1234, Name: "batguicore.exe"}}, act, 1, now.Add(time.Duration(i)*time.Second))
	}
	if _, ok := act[1300]; !ok {
		t.Fatal("launcher child lost trigger role")
	}
}

// BatGui exits, its ID is reused by a short-lived process that starts another
// program and exits too: that program must not become a trigger.
func TestChildOfReusedIDAfterExit(t *testing.T) {
	resetCaches()
	setCreated(1234, "batgui.exe", 900)
	now := time.Now()
	act := activeTriggers(batRule, []Proc{{PID: 1234, PPID: 4, Name: "batgui.exe"}}, nil, 1, now)
	// BatGui gone; 3 s later a new process appears with parent ID 1234.
	act = activeTriggers(batRule, nil, act, 1, now.Add(time.Second))
	setCreated(1500, "updater.exe", 5000)
	act = activeTriggers(batRule, []Proc{{PID: 1500, PPID: 1234, Name: "updater.exe"}}, act, 1, now.Add(3*time.Second))
	if _, ok := act[1500]; ok {
		t.Fatal("process that appeared after BatGui exited became a trigger")
	}
}

// The user's sequence: BatGui closes Nextcloud, Nextcloud is started again
// while BatGui runs, BatGui is closed: afterwards nothing may be active.
func TestNothingLeftAfterBatGuiCloses(t *testing.T) {
	resetCaches()
	setCreated(10, "explorer.exe", 1)
	setCreated(1234, "batgui.exe", 900)
	setCreated(2000, "nextcloud.exe", 950)
	setCreated(2100, "nextcloud.exe", 1200)
	now := time.Now()
	procs := []Proc{{PID: 10, Name: "explorer.exe"}, {PID: 1234, PPID: 10, Name: "batgui.exe"}, {PID: 2000, PPID: 10, Name: "nextcloud.exe"}}
	act := activeTriggers(batRule, procs, nil, 1, now)
	procs = []Proc{{PID: 10, Name: "explorer.exe"}, {PID: 1234, PPID: 10, Name: "batgui.exe"}, {PID: 2100, PPID: 10, Name: "nextcloud.exe"}}
	act = activeTriggers(batRule, procs, act, 1, now.Add(5*time.Second))
	procs = []Proc{{PID: 10, Name: "explorer.exe"}}
	for i := 6; i < 30; i++ {
		act = activeTriggers(batRule, procs, act, 1, now.Add(time.Duration(i)*time.Second))
	}
	for pid := range act {
		if pid == 10 {
			t.Fatal("explorer.exe became a trigger")
		}
	}
}

func setSystem(pid int, name string) {
	systemCache[pid] = struct {
		name string
		sys  bool
	}{name, true}
}

// The reported case: BatGui (running for minutes) makes Windows start
// splwow64.exe, which keeps running after BatGui is closed.
func TestSplwow64DoesNotBlock(t *testing.T) {
	resetCaches()
	setCreated(1234, "BatGui.exe", 900)
	setCreated(3000, "splwow64.exe", 1000)
	setSystem(3000, "splwow64.exe")
	now := time.Now()
	procs := []Proc{{PID: 1234, PPID: 10, Name: "BatGui.exe"}, {PID: 3000, PPID: 1234, Name: "splwow64.exe"}}
	var act map[int]triggerProc
	for i := 0; i < 120; i++ {
		act = activeTriggers(batRule, procs, act, 1, now.Add(time.Duration(i)*time.Second))
	}
	if _, ok := act[3000]; ok {
		t.Fatal("splwow64.exe counted as trigger")
	}
}

// A non-system helper left behind by a long-running trigger stops counting
// once the trigger is closed.
func TestLeftoverOfLongRunningTrigger(t *testing.T) {
	resetCaches()
	setCreated(1234, "batgui.exe", 900)
	setCreated(3000, "helper.exe", 1000)
	now := time.Now()
	procs := []Proc{{PID: 1234, PPID: 10, Name: "batgui.exe"}, {PID: 3000, PPID: 1234, Name: "helper.exe"}}
	var act map[int]triggerProc
	for i := 0; i < 60; i++ { // BatGui runs for a minute
		act = activeTriggers(batRule, procs, act, 1, now.Add(time.Duration(i)*time.Second))
	}
	if _, ok := act[3000]; !ok {
		t.Fatal("helper should count while BatGui runs")
	}
	procs = []Proc{{PID: 3000, PPID: 1234, Name: "helper.exe"}}
	act = activeTriggers(batRule, procs, act, 1, now.Add(61*time.Second))
	if _, ok := act[3000]; ok {
		t.Fatal("leftover helper still counts after BatGui was closed")
	}
}

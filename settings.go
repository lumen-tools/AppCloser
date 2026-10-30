//go:build gui

package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ui struct {
	app      fyne.App
	win      fyne.Window
	rulesBox *fyne.Container
	status   *widget.Label
	pauseChk *widget.Check
	// live status line per rule card
	ruleStatus []*widget.Label
}

// runSettings shows the settings window. It runs as its own process that
// ends when the window is closed, so the GUI toolkit uses no resources while
// only the tray icon is active.
func runSettings() {
	u := &ui{}
	if !instanceLock("settings", func(msg string) {
		switch msg {
		case "show":
			fyne.Do(u.showWindow)
		case "quit":
			fyne.Do(u.closeWindow)
		}
	}) {
		sendInstance("settings", "show")
		return
	}
	isolateEnv()
	loadConfig()

	forceEnglish()
	u.app = app.NewWithID("de.appcloser")
	u.app.SetIcon(iconActive)
	u.win = u.app.NewWindow("AppCloser")
	u.win.SetIcon(iconActive)
	u.win.SetCloseIntercept(u.closeWindow)
	u.win.Resize(fyne.NewSize(900, 620))
	u.win.SetContent(u.build())
	u.refreshRules()
	u.refreshStatus()

	// Only computes the per-rule status; the tray (or service) enforces.
	go runMonitor(nil, reloadIfChanged, false, nil)
	go func() {
		for range time.Tick(500 * time.Millisecond) {
			fyne.Do(u.refreshLive)
		}
	}()

	u.win.Show()
	u.app.Run()
	os.Exit(0)
}

// closeWindow ends the settings process; the tray keeps running.
func (u *ui) closeWindow() {
	// The toolkit does not always end the process by itself, so force it.
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	u.app.Quit()
}

// quitAll ends AppCloser completely: tray, background service and window.
func (u *ui) quitAll() {
	sendInstance("tray", "quit")
	stopBackground()
	u.win.Hide()
	go func() {
		endAll()
		os.Exit(0)
	}()
}

func (u *ui) showWindow() {
	u.win.Show()
	u.win.RequestFocus()
}

func (u *ui) setPaused(p bool) {
	updateConfig(func(c *Config) { c.Paused = p })
	u.refreshStatus()
}

func (u *ui) refreshStatus() {
	c := snapshot()
	active := 0
	for _, r := range c.Rules {
		if r.Enabled && r.Target.Match != "" && len(r.Triggers) > 0 {
			active++
		}
	}
	switch {
	case c.Paused:
		u.status.SetText("Monitoring paused – nothing will be closed.")
		u.win.SetIcon(iconPaused)
	case active == 0:
		u.status.SetText("No complete rule yet. Choose a program on the left and at least one trigger on the right.")
		u.win.SetIcon(iconActive)
	default:
		u.status.SetText("Monitoring active – " + plural(active, "rule", "rules") + " in effect.")
		u.win.SetIcon(iconActive)
	}
	if u.pauseChk != nil && u.pauseChk.Checked != c.Paused {
		u.pauseChk.SetChecked(c.Paused)
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return itoa(n) + " " + many
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func (u *ui) build() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("Rules", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	addRule := widget.NewButtonWithIcon("New rule", theme.ContentAddIcon(), func() {
		updateConfig(func(c *Config) { c.Rules = append(c.Rules, Rule{Enabled: true}) })
		u.refreshRules()
	})
	addRule.Importance = widget.HighImportance
	header := container.NewHBox(title, layout.NewSpacer(), addRule)

	u.rulesBox = container.NewVBox()
	scroll := container.NewVScroll(u.rulesBox)

	c := snapshot()
	auto := widget.NewCheck("Start with system", func(on bool) {
		if err := setAutostart(on); err != nil {
			dialog.ShowError(err, u.win)
		}
	})
	auto.SetChecked(autostartEnabled())
	autoHint := widget.NewLabel(autostartHint)
	autoHint.Wrapping = fyne.TextWrapWord
	autoHint.Importance = widget.LowImportance

	notes := widget.NewCheck("Show a notification when a program is closed", func(on bool) {
		updateConfig(func(c *Config) { c.Notifications = on })
	})
	notes.SetChecked(c.Notifications)

	u.pauseChk = widget.NewCheck("Pause monitoring", func(on bool) {
		if on != snapshot().Paused {
			u.setPaused(on)
		}
	})
	u.pauseChk.SetChecked(c.Paused)

	u.status = widget.NewLabel("")
	u.status.TextStyle = fyne.TextStyle{Bold: true}
	u.status.Wrapping = fyne.TextWrapWord

	hide := widget.NewButtonWithIcon("Close window", theme.VisibilityOffIcon(), u.closeWindow)
	exit := widget.NewButtonWithIcon("Quit AppCloser", theme.LogoutIcon(), u.quitAll)
	settings := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(auto, notes, u.pauseChk),
		autoHint,
		container.NewBorder(nil, nil, nil, container.NewHBox(hide, exit), u.status),
	)
	return container.NewBorder(header, settings, nil, nil, scroll)
}

// refreshLive shows per rule whether a trigger is currently detected.
func (u *ui) refreshLive() {
	states := currentStates()
	paused := snapshot().Paused
	if u.pauseChk.Checked != paused { // changed via the tray menu
		u.refreshStatus()
	}
	for i, l := range u.ruleStatus {
		text, imp := "No trigger running – the program may run.", widget.LowImportance
		if paused {
			text = "Paused"
		} else if i < len(states) && states[i].Trigger != "" {
			text, imp = "Trigger detected: "+states[i].Trigger+" – program is being blocked.", widget.HighImportance
		}
		if l.Text != text || l.Importance != imp {
			l.Importance = imp
			l.SetText(text)
		}
	}
}

func (u *ui) refreshRules() {
	u.rulesBox.RemoveAll()
	u.ruleStatus = nil
	rules := snapshot().Rules
	if len(rules) == 0 {
		empty := widget.NewLabel("No rules yet. Click “New rule” in the top right corner.")
		empty.Alignment = fyne.TextAlignCenter
		u.rulesBox.Add(container.NewPadded(empty))
	}
	for i := range rules {
		u.rulesBox.Add(u.ruleCard(i, rules[i]))
	}
	u.rulesBox.Refresh()
	if u.status != nil {
		u.refreshStatus()
	}
}

func (u *ui) edit(i int, f func(r *Rule)) {
	updateConfig(func(c *Config) {
		if i < len(c.Rules) {
			f(&c.Rules[i])
		}
	})
	u.refreshRules()
}

func (u *ui) ruleCard(i int, r Rule) fyne.CanvasObject {
	enabled := widget.NewCheck("Rule "+itoa(i+1)+" enabled", nil)
	enabled.SetChecked(r.Enabled)
	enabled.OnChanged = func(on bool) { u.edit(i, func(r *Rule) { r.Enabled = on }) }

	del := widget.NewButtonWithIcon("Delete rule", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Delete rule", "Really delete rule "+itoa(i+1)+"?", func(ok bool) {
			if ok {
				updateConfig(func(c *Config) {
					if i < len(c.Rules) {
						c.Rules = append(c.Rules[:i], c.Rules[i+1:]...)
					}
				})
				u.refreshRules()
			}
		}, u.win)
	})
	del.Importance = widget.LowImportance
	top := container.NewHBox(enabled, layout.NewSpacer(), del)

	// Left: program to close.
	var targetLbl *widget.Label
	if r.Target.Match == "" {
		targetLbl = widget.NewLabel("– not selected yet –")
		targetLbl.Importance = widget.LowImportance
	} else {
		targetLbl = widget.NewLabelWithStyle(r.Target.Display(), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	targetLbl.Wrapping = fyne.TextWrapBreak
	choose := widget.NewButtonWithIcon("Choose program…", theme.SearchIcon(), func() {
		u.pick("Which program should be closed?", func(p Program) {
			u.edit(i, func(r *Rule) { r.Target = p })
		})
	})
	left := widget.NewCard("", "Immediately close this program …",
		container.NewVBox(targetLbl, container.NewHBox(choose)))

	// Right: triggers.
	trig := container.NewVBox()
	if len(r.Triggers) == 0 {
		l := widget.NewLabel("– no trigger yet –")
		l.Importance = widget.LowImportance
		trig.Add(l)
	}
	for j, t := range r.Triggers {
		j := j
		l := widget.NewLabel(t.Display())
		l.Wrapping = fyne.TextWrapBreak
		rm := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
			u.edit(i, func(r *Rule) {
				if j < len(r.Triggers) {
					r.Triggers = append(r.Triggers[:j], r.Triggers[j+1:]...)
				}
			})
		})
		rm.Importance = widget.LowImportance
		trig.Add(container.NewBorder(nil, nil, nil, rm, l))
	}
	addT := widget.NewButtonWithIcon("Add trigger…", theme.ContentAddIcon(), func() {
		u.pick("Which program should trigger the closing?", func(p Program) {
			u.edit(i, func(r *Rule) {
				for _, t := range r.Triggers {
					if strings.EqualFold(t.Match, p.Match) {
						return
					}
				}
				r.Triggers = append(r.Triggers, p)
			})
		})
	})
	right := widget.NewCard("", "… as soon as one of these programs is running",
		container.NewVBox(trig, container.NewHBox(addT)))

	body := container.NewGridWithColumns(2, left, right)
	live := widget.NewLabel("")
	live.Importance = widget.LowImportance
	live.Wrapping = fyne.TextWrapWord
	u.ruleStatus = append(u.ruleStatus, live)
	return container.NewVBox(top, body, live, widget.NewSeparator())
}

// pick shows a searchable list of running processes (and flatpak apps on Linux).
func (u *ui) pick(title string, done func(Program)) {
	all := pickerEntries()
	filtered := all
	selected := -1

	search := widget.NewEntry()
	search.SetPlaceHolder("Search or type a program name, e.g. nextcloud.exe")

	list := widget.NewList(
		func() int { return len(filtered) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(filtered[id].Display()) },
	)
	list.OnSelected = func(id widget.ListItemID) { selected = id }

	search.OnChanged = func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		filtered = nil
		for _, p := range all {
			if s == "" || strings.Contains(strings.ToLower(p.Display()), s) {
				filtered = append(filtered, p)
			}
		}
		selected = -1
		list.UnselectAll()
		list.Refresh()
	}

	hint := widget.NewLabel(pickerHint)
	hint.Wrapping = fyne.TextWrapWord
	hint.Importance = widget.LowImportance

	var d dialog.Dialog
	finish := func(p Program) {
		d.Hide()
		done(p)
	}
	ok := widget.NewButtonWithIcon("Select", theme.ConfirmIcon(), func() {
		if selected >= 0 && selected < len(filtered) {
			finish(filtered[selected])
			return
		}
		if t := strings.TrimSpace(search.Text); t != "" {
			finish(Program{Label: t, Match: t})
		}
	})
	ok.Importance = widget.HighImportance
	list.OnSelected = func(id widget.ListItemID) { selected = id }
	search.OnSubmitted = func(string) { ok.OnTapped() }

	file := widget.NewButtonWithIcon("Choose file…", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			name := filepath.Base(rc.URI().Path())
			rc.Close()
			finish(Program{Label: name, Match: name})
		}, u.win)
		if home, err := storage.ListerForURI(storage.NewFileURI(origHome)); err == nil {
			fd.SetLocation(home)
		}
		fd.Resize(fyne.NewSize(800, 550))
		fd.Show()
	})
	cancel := widget.NewButton("Cancel", func() { d.Hide() })
	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		all = pickerEntries()
		search.OnChanged(search.Text)
	})

	buttons := container.NewHBox(file, refresh, layout.NewSpacer(), cancel, ok)
	content := container.NewBorder(container.NewVBox(search, hint), buttons, nil, nil, list)
	d = dialog.NewCustomWithoutButtons(title, content, u.win)
	d.Resize(fyne.NewSize(700, 520))
	d.Show()
	u.win.Canvas().Focus(search)
}

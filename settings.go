//go:build gui

package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ui struct {
	app       fyne.App
	win       fyne.Window
	rulesBox  *fyne.Container
	status    *label
	statusDot *shape
	pauseSw   *toggle
	// live status line per rule card
	ruleStatus []*label
	ruleDots   []*shape
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
	u.app.Settings().SetTheme(appTheme{})
	u.app.SetIcon(iconActive)
	u.win = u.app.NewWindow("AppCloser Settings")
	u.win.SetIcon(iconActive)
	u.win.SetCloseIntercept(u.closeWindow)
	u.fitWindow(0)
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
	// The real display scale is known once the window exists; correct the
	// size if it differs from the estimate.
	go func() {
		time.Sleep(150 * time.Millisecond)
		fyne.Do(func() { u.fitWindow(u.win.Canvas().Scale()) })
	}()
	u.app.Run()
	os.Exit(0)
}

// fitWindow sizes the window to its preferred size, but never larger than
// the free screen area (the content scrolls instead), and centers it.
func (u *ui) fitWindow(scale float32) {
	size := fyne.NewSize(720, 760)
	aw, ah, est := workArea()
	if scale <= 0 {
		scale = est
	}
	if aw > 0 && ah > 0 && scale > 0 {
		// Leave room for the window frame and title bar.
		size.Width = min(size.Width, aw/scale-32)
		size.Height = min(size.Height, ah/scale-64)
	}
	if u.win.Canvas().Size() == size {
		return
	}
	u.win.Resize(size)
	u.win.CenterOnScreen()
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
		u.status.Set("Paused — nothing is closed", colorSecondary)
		u.statusDot.fill = colorSecondary
		u.win.SetIcon(iconPaused)
	case active == 0:
		u.status.Set("No active rules", colorSecondary)
		u.statusDot.fill = colorSecondary
		u.win.SetIcon(iconActive)
	default:
		u.status.Set("Monitoring · "+plural(active, "rule", "rules")+" active", theme.ColorNameSuccess)
		u.statusDot.fill = theme.ColorNameSuccess
		u.win.SetIcon(iconActive)
	}
	u.statusDot.Refresh()
	if u.pauseSw != nil {
		u.pauseSw.SetOn(c.Paused)
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
	c := snapshot()

	// Header: app name, overall status and the one primary action.
	logo := canvas.NewImageFromResource(iconActive)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(40, 40))
	u.status = newLabel("", theme.SizeNameText, colorSecondary, false)
	u.statusDot = dot(colorSecondary)
	titleBlock := tight(4,
		newLabel("AppCloser", sizeTitle, theme.ColorNameForeground, true),
		container.NewBorder(nil, nil, container.NewCenter(u.statusDot), nil, u.status),
	)
	addRule := widget.NewButtonWithIcon("Add Rule", theme.ContentAddIcon(), u.addRule)
	addRule.Importance = widget.HighImportance
	header := container.NewBorder(nil, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 12), container.NewCenter(logo)),
		container.NewCenter(addRule), vcenter(titleBlock))

	u.rulesBox = container.NewVBox()

	// General settings: rarely changed options in one grouped form.
	auto := newToggle(autostartEnabled(), nil)
	auto.OnChanged = func(on bool) {
		if err := setAutostart(on); err != nil {
			auto.SetOn(!on)
			dialog.ShowError(err, u.win)
		}
	}
	notes := newToggle(c.Notifications, func(on bool) {
		updateConfig(func(c *Config) { c.Notifications = on })
	})
	u.pauseSw = newToggle(c.Paused, func(on bool) {
		if on != snapshot().Paused {
			u.setPaused(on)
		}
	})
	general := card(
		settingRow("Start with System", autostartHint, auto),
		divider(),
		settingRow("Notifications", "Show a notification when AppCloser closes a program.", notes),
		divider(),
		settingRow("Pause Monitoring", "While paused, nothing is closed.", u.pauseSw),
	)

	quit := widget.NewButtonWithIcon("Quit AppCloser", theme.LogoutIcon(), u.quitAll)
	footer := container.NewBorder(nil, nil, nil, quit,
		paragraph("Closing this window keeps AppCloser running in the tray.", sizeCaption, colorSecondary, false))

	content := container.NewVBox(
		header,
		sectionHeader("RULES"),
		u.rulesBox,
		sectionHeader("GENERAL"),
		general,
		container.New(layout.NewCustomPaddedLayout(14, 4, 0, 0), footer),
	)
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(20, 20, 24, 24), content))
}

func (u *ui) addRule() {
	updateConfig(func(c *Config) { c.Rules = append(c.Rules, Rule{Enabled: true}) })
	u.refreshRules()
	// A new rule starts with choosing the program to close.
	i := len(snapshot().Rules) - 1
	u.pick("Choose the App to Close", func(p Program) {
		u.edit(i, func(r *Rule) { r.Target = p })
	})
}

// refreshLive shows per rule whether a trigger is currently detected.
func (u *ui) refreshLive() {
	states := currentStates()
	c := snapshot()
	if u.pauseSw.On != c.Paused { // changed via the tray menu
		u.refreshStatus()
	}
	for i, l := range u.ruleStatus {
		msg, col := "Waiting — none of these is running", colorSecondary
		if i < len(c.Rules) {
			r := c.Rules[i]
			switch {
			case r.Target.Match == "":
				msg = "Choose the app to close"
			case len(r.Triggers) == 0:
				msg = "Add at least one trigger"
			case !r.Enabled:
				msg = "Rule is off"
			case c.Paused:
				msg = "Paused"
			case i < len(states) && states[i].Trigger != "":
				msg, col = "Blocking now — "+states[i].Trigger+" is running", colorActive
			}
		}
		l.Set(msg, col)
		if d := u.ruleDots[i]; d.fill != col {
			d.fill = col
			d.Refresh()
		}
	}
}

func (u *ui) refreshRules() {
	u.rulesBox.RemoveAll()
	u.ruleStatus, u.ruleDots = nil, nil
	rules := snapshot().Rules
	if len(rules) == 0 {
		add := widget.NewButtonWithIcon("Add Rule", theme.ContentAddIcon(), u.addRule)
		add.Importance = widget.HighImportance
		title := container.NewCenter(newLabel("No Rules Yet", sizeRuleTitle, theme.ColorNameForeground, true))
		hint := paragraph("A rule closes one app while another app is running — for example, a cloud sync client while a game runs.",
			theme.SizeNameText, colorSecondary, true)
		u.rulesBox.Add(card(container.New(layout.NewCustomPaddedLayout(16, 16, 24, 24),
			tight(10, title, hint, container.NewCenter(add)))))
	}
	for i := range rules {
		if i > 0 {
			u.rulesBox.Add(gap(6))
		}
		u.rulesBox.Add(u.ruleCard(i, rules[i]))
	}
	u.rulesBox.Refresh()
	if u.status != nil {
		u.refreshStatus()
		u.refreshLive()
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

// sentence describes a rule in plain words, e.g.
// "Closes while RetroBat or RetroDECK is running."
func sentence(r Rule) string {
	var names []string
	for _, t := range r.Triggers {
		names = append(names, progName(t))
	}
	switch len(names) {
	case 0:
		return "Add the apps that should close it."
	case 1:
		return "Closes while " + names[0] + " is running."
	default:
		return "Closes while " + strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1] + " is running."
	}
}

func kind(p Program) string {
	if strings.HasPrefix(p.Match, "flatpak:") {
		return "Flatpak · " + strings.TrimPrefix(p.Match, "flatpak:")
	}
	if p.Label != "" && !strings.EqualFold(p.Label, p.Match) {
		return p.Match
	}
	return ""
}

func (u *ui) ruleCard(i int, r Rule) fyne.CanvasObject {
	// Header row: the app to close, the rule in one sentence, and its switch.
	var head fyne.CanvasObject
	if r.Target.Match == "" {
		choose := widget.NewButtonWithIcon("Choose App to Close…", theme.SearchIcon(), func() {
			u.pick("Choose the App to Close", func(p Program) { u.edit(i, func(r *Rule) { r.Target = p }) })
		})
		choose.Importance = widget.HighImportance
		head = container.NewBorder(nil, nil, container.NewCenter(monogram("", 34)), nil,
			container.NewHBox(container.NewCenter(choose)))
	} else {
		titles := tight(3,
			newLabel(progName(r.Target), sizeRuleTitle, theme.ColorNameForeground, true),
			paragraph(sentence(r), theme.SizeNameText, colorSecondary, false),
		)
		sw := newToggle(r.Enabled, func(on bool) { u.edit(i, func(r *Rule) { r.Enabled = on }) })
		head = container.NewBorder(nil, nil,
			container.New(layout.NewCustomPaddedLayout(0, 0, 0, 12), container.NewCenter(monogram(progName(r.Target), 34))),
			container.New(layout.NewCustomPaddedLayout(0, 0, 16, 0), container.NewCenter(sw)),
			vcenter(titles))
	}

	rows := []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(6, 6, 0, 0), head), divider()}

	// Trigger rows.
	for j, t := range r.Triggers {
		j := j
		var labels fyne.CanvasObject = newLabel(progName(t), theme.SizeNameText, theme.ColorNameForeground, false)
		if k := kind(t); k != "" {
			labels = tight(2, labels, newLabel(k, sizeCaption, colorSecondary, false))
		}
		rm := plainButton("", theme.ContentRemoveIcon(), func() {
			u.edit(i, func(r *Rule) {
				if j < len(r.Triggers) {
					r.Triggers = append(r.Triggers[:j], r.Triggers[j+1:]...)
				}
			})
		})
		rows = append(rows, container.New(layout.NewCustomPaddedLayout(4, 4, 0, 0), container.NewBorder(nil, nil,
			container.New(layout.NewCustomPaddedLayout(0, 0, 10, 10), container.NewCenter(monogram(progName(t), 24))),
			container.NewCenter(rm), vcenter(labels))))
	}
	addT := plainButton("Add Trigger…", theme.ContentAddIcon(), func() {
		u.pick("Choose an App That Triggers Closing", func(p Program) {
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
	rows = append(rows, container.NewHBox(addT), divider())

	// Footer: live status and the less frequent actions.
	live := newLabel("", sizeCaption, colorSecondary, false)
	d := dot(colorSecondary)
	u.ruleStatus = append(u.ruleStatus, live)
	u.ruleDots = append(u.ruleDots, d)
	change := plainButton("Change App…", nil, func() {
		u.pick("Choose the App to Close", func(p Program) { u.edit(i, func(r *Rule) { r.Target = p }) })
	})
	del := plainButton("Delete", theme.DeleteIcon(), func() {
		name := progName(r.Target)
		if name == "" {
			name = "this rule"
		}
		dialog.ShowConfirm("Delete Rule?", "The rule for "+name+" will be removed.", func(ok bool) {
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
	actions := container.NewHBox(del)
	if r.Target.Match != "" {
		actions = container.NewHBox(change, del)
	}
	rows = append(rows, container.NewBorder(nil, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), container.NewCenter(d)), actions, live))
	return card(rows...)
}

// pick shows a searchable list of running processes (and flatpak apps on Linux).
func (u *ui) pick(title string, done func(Program)) {
	all := pickerList()
	filtered := all
	selected := -1

	search := widget.NewEntry()
	search.SetPlaceHolder("Search, or type a program name such as nextcloud.exe")
	search.ActionItem = widget.NewIcon(theme.SearchIcon())

	list := widget.NewList(
		func() int { return len(filtered) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, container.NewStack(), nil,
				vcenter(tight(2, newLabel("", theme.SizeNameText, theme.ColorNameForeground, false),
					newLabel("", sizeCaption, colorSecondary, false))))
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			p := filtered[id]
			row := o.(*fyne.Container)
			labels := row.Objects[0].(*fyne.Container).Objects[0].(*fyne.Container)
			badge := row.Objects[1].(*fyne.Container)
			labels.Objects[0].(*label).Set(progName(p), theme.ColorNameForeground)
			k := labels.Objects[1].(*label)
			k.Set(kind(p), colorSecondary)
			if kind(p) == "" {
				k.Hide()
			} else {
				k.Show()
			}
			badge.Objects = []fyne.CanvasObject{container.New(layout.NewCustomPaddedLayout(0, 0, 0, 10), container.NewCenter(monogram(progName(p), 26)))}
			badge.Refresh()
		},
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

	hint := paragraph(pickerHint, sizeCaption, colorSecondary, false)

	var d dialog.Dialog
	finish := func(p Program) {
		d.Hide()
		done(p)
	}
	ok := widget.NewButton("Select", func() {
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

	file := plainButton("Choose File…", theme.FolderOpenIcon(), func() {
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
	refresh := plainButton("", theme.ViewRefreshIcon(), func() {
		all = pickerList()
		search.OnChanged(search.Text)
	})

	buttons := container.NewHBox(file, refresh, layout.NewSpacer(), cancel, ok)
	content := container.NewBorder(container.NewVBox(search, hint), buttons, nil, nil, list)
	d = dialog.NewCustomWithoutButtons(title, content, u.win)
	d.Resize(fyne.NewSize(620, 560))
	d.Show()
	u.win.Canvas().Focus(search)
}

// pickerList is pickerEntries without AppCloser's own programs.
func pickerList() []Program {
	var out []Program
	for _, p := range pickerEntries() {
		if n := normName(p.Match); n != normName(trayExe) && n != normName(settingsExe) {
			out = append(out, p)
		}
	}
	return out
}

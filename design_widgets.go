//go:build gui

package main

import (
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// --- Text ---

// label is a single line of text without padding, in a theme size and color.
type label struct {
	widget.BaseWidget
	Text     string
	TextSize fyne.ThemeSizeName
	Color    fyne.ThemeColorName
	Bold     bool
	obj      *canvas.Text
}

func newLabel(s string, size fyne.ThemeSizeName, col fyne.ThemeColorName, bold bool) *label {
	l := &label{Text: s, TextSize: size, Color: col, Bold: bold, obj: canvas.NewText(s, color.Black)}
	l.ExtendBaseWidget(l)
	l.apply()
	return l
}

func (l *label) apply() {
	l.obj.Text, l.obj.TextSize, l.obj.Color = l.Text, theme.Size(l.TextSize), theme.Color(l.Color)
	l.obj.TextStyle = fyne.TextStyle{Bold: l.Bold}
}

func (l *label) Set(s string, col fyne.ThemeColorName) {
	if l.Text == s && l.Color == col {
		return
	}
	l.Text, l.Color = s, col
	l.Refresh()
}

func (l *label) MinSize() fyne.Size { l.apply(); return l.obj.MinSize() }

func (l *label) CreateRenderer() fyne.WidgetRenderer {
	return &labelRenderer{l}
}

type labelRenderer struct{ l *label }

func (r *labelRenderer) Layout(sz fyne.Size) {
	m := r.l.obj.MinSize()
	r.l.obj.Move(fyne.NewPos(0, (sz.Height-m.Height)/2))
	r.l.obj.Resize(fyne.NewSize(sz.Width, m.Height))
}
func (r *labelRenderer) MinSize() fyne.Size           { return r.l.MinSize() }
func (r *labelRenderer) Refresh()                     { r.l.apply(); r.l.obj.Refresh() }
func (r *labelRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.l.obj} }
func (r *labelRenderer) Destroy()                     {}

// paragraph is wrapping text; the toolkit's own padding is removed so it
// lines up with labels.
func paragraph(s string, size fyne.ThemeSizeName, col fyne.ThemeColorName, center bool) fyne.CanvasObject {
	seg := &widget.TextSegment{Text: s, Style: widget.RichTextStyle{ColorName: col, SizeName: size, Inline: true}}
	if center {
		seg.Style.Alignment = fyne.TextAlignCenter
	}
	t := widget.NewRichText(seg)
	t.Wrapping = fyne.TextWrapWord
	p := theme.Size(theme.SizeNameInnerPadding)
	return container.New(layout.NewCustomPaddedLayout(-p/2, -p/2, -p, -p), t)
}

// tight stacks objects vertically with a small fixed gap.
func tight(gap float32, objs ...fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&tightLayout{gap}, objs...)
}

type tightLayout struct{ gap float32 }

func (t *tightLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	n := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w = max(w, m.Width)
		h += m.Height
		n++
	}
	if n > 1 {
		h += t.gap * float32(n-1)
	}
	return fyne.NewSize(w, h)
}

func (t *tightLayout) Layout(objs []fyne.CanvasObject, sz fyne.Size) {
	y := float32(0)
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(sz.Width, m.Height))
		y += m.Height + t.gap
	}
}

// gap is empty vertical space.
func gap(h float32) fyne.CanvasObject {
	return newShape(theme.ColorNameBackground, "", fyne.NewSize(1, h))
}

// --- Surfaces that follow the light/dark appearance ---

// shape is a rectangle or circle filled with a theme color, refreshed when the
// appearance changes.
type shape struct {
	widget.BaseWidget
	fill    fyne.ThemeColorName
	radius  fyne.ThemeSizeName // empty: no rounding
	circle  bool
	minSize fyne.Size
}

func newShape(fill fyne.ThemeColorName, radius fyne.ThemeSizeName, min fyne.Size) *shape {
	s := &shape{fill: fill, radius: radius, minSize: min}
	s.ExtendBaseWidget(s)
	return s
}

func (s *shape) MinSize() fyne.Size { return s.minSize }

func (s *shape) CreateRenderer() fyne.WidgetRenderer {
	if s.circle {
		c := canvas.NewCircle(color.Transparent)
		cr := &shapeRenderer{s: s, obj: c, apply: func() { c.FillColor = theme.Color(s.fill); c.Refresh() }}
		cr.apply()
		return cr
	}
	r := canvas.NewRectangle(color.Transparent)
	sr := &shapeRenderer{s: s, obj: r, apply: func() {
		r.FillColor = theme.Color(s.fill)
		if s.radius != "" {
			r.CornerRadius = theme.Size(s.radius)
		}
		r.Refresh()
	}}
	sr.apply()
	return sr
}

type shapeRenderer struct {
	s     *shape
	obj   fyne.CanvasObject
	apply func()
}

func (r *shapeRenderer) Layout(sz fyne.Size)          { r.obj.Resize(sz) }
func (r *shapeRenderer) MinSize() fyne.Size           { return r.s.minSize }
func (r *shapeRenderer) Refresh()                     { r.apply() }
func (r *shapeRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.obj} }
func (r *shapeRenderer) Destroy()                     {}

// card groups rows on a rounded surface, like a grouped form on macOS.
func card(rows ...fyne.CanvasObject) fyne.CanvasObject {
	p := theme.Size(sizeCardPadding)
	return container.NewStack(
		newShape(colorCard, sizeCardRadius, fyne.Size{}),
		container.New(layout.NewCustomPaddedLayout(p/2, p/2, p, p), container.NewVBox(rows...)),
	)
}

// divider is a hairline between rows inside a card.
func divider() fyne.CanvasObject {
	return newShape(theme.ColorNameSeparator, "", fyne.NewSize(1, 1))
}

// sectionHeader labels a group of cards, aligned with the card content.
func sectionHeader(s string) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(theme.Size(sizeSectionSpacing)-8, 0, theme.Size(sizeCardPadding)-6, 0),
		newLabel(s, sizeCaption, colorSecondary, true))
}

func dot(col fyne.ThemeColorName) *shape {
	d := newShape(col, "", fyne.NewSize(8, 8))
	d.circle = true
	return d
}

// --- Monogram: a colored badge with the program's initial ---

var monogramColors = []color.NRGBA{
	hex("#0A60D6"), hex("#5E5CE6"), hex("#0E7C86"), hex("#B25000"), hex("#B0306A"), hex("#2F7D32"), hex("#6B5B95"),
}

func monogram(name string, size float32) fyne.CanvasObject {
	name = strings.TrimSpace(strings.TrimPrefix(name, "flatpak:"))
	initial, sum := "?", uint32(2166136261)
	for _, r := range strings.ToLower(name) {
		sum = (sum ^ uint32(r)) * 16777619 // FNV-1a
	}
	if name != "" {
		initial = strings.ToUpper(string([]rune(name)[0]))
	}
	bg := canvas.NewRectangle(monogramColors[sum%uint32(len(monogramColors))])
	bg.CornerRadius = size * 0.27
	if name == "" {
		bg.FillColor = hex("#8E8E93")
	}
	letter := canvas.NewText(initial, color.White)
	letter.TextStyle = fyne.TextStyle{Bold: true}
	letter.TextSize = size * 0.46
	letter.Alignment = fyne.TextAlignCenter
	box := container.NewStack(bg, container.NewCenter(letter))
	return container.NewGridWrap(fyne.NewSize(size, size), box)
}

// --- Switch: a macOS-style toggle (keyboard: Tab to focus, Space to toggle) ---

type toggle struct {
	widget.BaseWidget
	On        bool
	OnChanged func(bool)
	focused   bool
	hovered   bool
	pos       float32 // knob position 0..1, animated
}

func newToggle(on bool, changed func(bool)) *toggle {
	t := &toggle{On: on, OnChanged: changed}
	if on {
		t.pos = 1
	}
	t.ExtendBaseWidget(t)
	return t
}

func (t *toggle) SetOn(on bool) {
	if t.On == on {
		return
	}
	t.On = on
	if on {
		t.pos = 1
	} else {
		t.pos = 0
	}
	t.Refresh()
}

func (t *toggle) flip() {
	t.On = !t.On
	from, to := t.pos, float32(0)
	if t.On {
		to = 1
	}
	fyne.NewAnimation(140*time.Millisecond, func(f float32) {
		t.pos = from + (to-from)*f
		t.Refresh()
	}).Start()
	if t.OnChanged != nil {
		t.OnChanged(t.On)
	}
}

func (t *toggle) Tapped(*fyne.PointEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(t); c != nil {
		c.Focus(t)
	}
	t.flip()
}
func (t *toggle) Cursor() desktop.Cursor         { return desktop.PointerCursor }
func (t *toggle) MouseIn(*desktop.MouseEvent)    { t.hovered = true; t.Refresh() }
func (t *toggle) MouseMoved(*desktop.MouseEvent) {}
func (t *toggle) MouseOut()                      { t.hovered = false; t.Refresh() }
func (t *toggle) FocusGained()                   { t.focused = true; t.Refresh() }
func (t *toggle) FocusLost()                     { t.focused = false; t.Refresh() }
func (t *toggle) TypedRune(r rune) {
	if r == ' ' {
		t.flip()
	}
}
func (t *toggle) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		t.flip()
	}
}
func (t *toggle) MinSize() fyne.Size { return fyne.NewSize(38, 22) }

func (t *toggle) CreateRenderer() fyne.WidgetRenderer {
	r := &toggleRenderer{t: t, track: canvas.NewRectangle(color.Transparent), knob: canvas.NewCircle(color.White)}
	r.knob.StrokeWidth = 0.5
	r.Refresh()
	return r
}

type toggleRenderer struct {
	t     *toggle
	track *canvas.Rectangle
	knob  *canvas.Circle
}

func (r *toggleRenderer) MinSize() fyne.Size { return r.t.MinSize() }

func (r *toggleRenderer) Layout(sz fyne.Size) {
	m := r.t.MinSize()
	x, y := float32(0), (sz.Height-m.Height)/2
	r.track.Move(fyne.NewPos(x, y))
	r.track.Resize(m)
	r.track.CornerRadius = m.Height / 2
	k := m.Height - 4
	r.knob.Move(fyne.NewPos(x+2+(m.Width-m.Height)*r.t.pos, y+2))
	r.knob.Resize(fyne.NewSize(k, k))
}

func (r *toggleRenderer) Refresh() {
	off, on := theme.Color(colorSwitchOff), theme.Color(theme.ColorNamePrimary)
	r.track.FillColor = blend(off, on, r.t.pos)
	if r.t.hovered && !r.t.On {
		r.track.FillColor = blend(off, theme.Color(colorSecondary), 0.25)
	}
	r.track.StrokeWidth = 0
	if r.t.focused {
		r.track.StrokeColor = theme.Color(theme.ColorNameFocus)
		r.track.StrokeWidth = 2
	}
	r.knob.FillColor = theme.Color(colorKnob)
	r.knob.StrokeColor = theme.Color(theme.ColorNameShadow)
	r.Layout(r.t.Size())
	r.track.Refresh()
	r.knob.Refresh()
}

func (r *toggleRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.knob}
}
func (r *toggleRenderer) Destroy() {}

func blend(a, b color.Color, f float32) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	mix := func(x, y uint32) uint8 { return uint8((float32(x)*(1-f) + float32(y)*f) / 257) }
	return color.NRGBA{mix(ar, br), mix(ag, bg), mix(ab, bb), 0xff}
}

// --- Rows ---

// settingRow is a title with an optional hint and a switch on the trailing side.
func settingRow(title, hint string, sw *toggle) fyne.CanvasObject {
	objs := []fyne.CanvasObject{newLabel(title, theme.SizeNameText, theme.ColorNameForeground, false)}
	if hint != "" {
		objs = append(objs, paragraph(hint, sizeCaption, colorSecondary, false))
	}
	return container.New(layout.NewCustomPaddedLayout(8, 8, 0, 0),
		container.NewBorder(nil, nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 16, 0), container.NewCenter(sw)), vcenter(tight(3, objs...))))
}

// plainButton is a borderless button in the accent (or another) color.
func plainButton(label string, icon fyne.Resource, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon(label, icon, tapped)
	b.Importance = widget.LowImportance
	return b
}

// vcenter keeps an object at full width and centers it vertically.
func vcenter(o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(vcenterLayout{}, o)
}

type vcenterLayout struct{}

func (vcenterLayout) MinSize(objs []fyne.CanvasObject) fyne.Size { return objs[0].MinSize() }

func (vcenterLayout) Layout(objs []fyne.CanvasObject, sz fyne.Size) {
	h := objs[0].MinSize().Height
	objs[0].Move(fyne.NewPos(0, (sz.Height-h)/2))
	objs[0].Resize(fyne.NewSize(sz.Width, h))
}

//go:build gui

package main

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Inter (SIL Open Font License, see fonts/Inter-LICENSE.txt) is close to the
// system typeface of Apple platforms and reads well on Windows and Linux.
var (
	//go:embed fonts/Inter-Regular.ttf
	interRegular []byte
	//go:embed fonts/Inter-SemiBold.ttf
	interSemiBold []byte

	fontRegular = fyne.NewStaticResource("Inter-Regular.ttf", interRegular)
	fontBold    = fyne.NewStaticResource("Inter-SemiBold.ttf", interSemiBold)
)

// Custom color and size names used by the settings window.
const (
	colorCard          fyne.ThemeColorName = "appcloser.card"
	colorSecondary     fyne.ThemeColorName = "appcloser.secondary"
	colorAccentText    fyne.ThemeColorName = "appcloser.accentText"
	colorActive        fyne.ThemeColorName = "appcloser.active"
	colorDanger        fyne.ThemeColorName = "appcloser.danger"
	colorSwitchOff     fyne.ThemeColorName = "appcloser.switchOff"
	colorKnob          fyne.ThemeColorName = "appcloser.knob"
	sizeTitle          fyne.ThemeSizeName  = "appcloser.title"
	sizeRuleTitle      fyne.ThemeSizeName  = "appcloser.ruleTitle"
	sizeCaption        fyne.ThemeSizeName  = "appcloser.caption"
	sizeCardRadius     fyne.ThemeSizeName  = "appcloser.cardRadius"
	sizeCardPadding    fyne.ThemeSizeName  = "appcloser.cardPadding"
	sizeSectionSpacing fyne.ThemeSizeName  = "appcloser.sectionSpacing"
)

func hex(s string) color.NRGBA {
	var c color.NRGBA
	for i, p := range []*uint8{&c.R, &c.G, &c.B} {
		v := 0
		for _, ch := range s[1+2*i : 3+2*i] {
			v *= 16
			switch {
			case ch >= '0' && ch <= '9':
				v += int(ch - '0')
			case ch >= 'a' && ch <= 'f':
				v += int(ch-'a') + 10
			case ch >= 'A' && ch <= 'F':
				v += int(ch-'A') + 10
			}
		}
		*p = uint8(v)
	}
	c.A = 0xff
	return c
}

func alpha(c color.NRGBA, a uint8) color.NRGBA { c.A = a; return c }

// palette holds light and dark values. Text colors reach at least 4.5:1 on the
// card and window backgrounds in both appearances.
var palette = map[fyne.ThemeColorName][2]color.Color{
	theme.ColorNameBackground:          {hex("#F2F2F7"), hex("#1C1C1E")},
	colorCard:                          {hex("#FFFFFF"), hex("#2C2C2E")},
	theme.ColorNameForeground:          {hex("#1D1D1F"), hex("#F5F5F7")}, // 16.8:1 / 12.8:1 on card
	colorSecondary:                     {hex("#6E6E73"), hex("#A1A1A6")}, // 5.1:1 / 5.4:1 on card
	theme.ColorNamePlaceHolder:         {hex("#6E6E73"), hex("#A1A1A6")},
	theme.ColorNamePrimary:             {hex("#0A60D6"), hex("#0A60D6")}, // white text on it: 5.7:1
	colorAccentText:                    {hex("#0A60D6"), hex("#4C9BFF")}, // 5.7:1 / 4.9:1 on card
	theme.ColorNameForegroundOnPrimary: {hex("#FFFFFF"), hex("#FFFFFF")},
	colorActive:                        {hex("#B25000"), hex("#FFA640")}, // 5.2:1 / 7.2:1 on card
	colorDanger:                        {hex("#C4281C"), hex("#FF6B5E")}, // 5.7:1 / 5.0:1 on card
	theme.ColorNameError:               {hex("#C4281C"), hex("#FF6B5E")},
	theme.ColorNameWarning:             {hex("#B25000"), hex("#FFA640")},
	theme.ColorNameSuccess:             {hex("#248A3D"), hex("#30D158")},
	colorSwitchOff:                     {hex("#C7C7CC"), hex("#5A5A5E")},
	colorKnob:                          {hex("#FFFFFF"), hex("#F5F5F7")},
	theme.ColorNameButton:              {hex("#E8E8ED"), hex("#3A3A3C")},
	theme.ColorNameDisabledButton:      {hex("#F2F2F7"), hex("#2C2C2E")},
	theme.ColorNameDisabled:            {hex("#AEAEB2"), hex("#636366")},
	theme.ColorNameInputBackground:     {hex("#FFFFFF"), hex("#1C1C1E")},
	theme.ColorNameInputBorder:         {hex("#C7C7CC"), hex("#545458")},
	theme.ColorNameSeparator:           {hex("#E5E5EA"), hex("#3A3A3C")},
	theme.ColorNameOverlayBackground:   {hex("#FFFFFF"), hex("#2C2C2E")},
	theme.ColorNameMenuBackground:      {hex("#FFFFFF"), hex("#2C2C2E")},
	theme.ColorNameHeaderBackground:    {hex("#F2F2F7"), hex("#1C1C1E")},
	theme.ColorNameHover:               {alpha(hex("#000000"), 0x10), alpha(hex("#FFFFFF"), 0x14)},
	theme.ColorNamePressed:             {alpha(hex("#000000"), 0x20), alpha(hex("#FFFFFF"), 0x28)},
	theme.ColorNameFocus:               {alpha(hex("#0A60D6"), 0x80), alpha(hex("#4C9BFF"), 0x90)},
	theme.ColorNameSelection:           {alpha(hex("#0A60D6"), 0x40), alpha(hex("#4C9BFF"), 0x50)},
	theme.ColorNameScrollBar:           {alpha(hex("#000000"), 0x50), alpha(hex("#FFFFFF"), 0x50)},
	theme.ColorNameShadow:              {alpha(hex("#000000"), 0x22), alpha(hex("#000000"), 0x80)},
	theme.ColorNameHyperlink:           {hex("#0A60D6"), hex("#4C9BFF")},
}

type appTheme struct{}

func (appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if c, ok := palette[n]; ok {
		if v == theme.VariantDark {
			return c[1]
		}
		return c[0]
	}
	return theme.DefaultTheme().Color(n, v)
}

func (appTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 14
	case theme.SizeNameCaptionText, sizeCaption:
		return 12
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameHeadingText, sizeTitle:
		return 24
	case sizeRuleTitle:
		return 16
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameInputBorder, theme.SizeNameSeparatorThickness:
		return 1
	case sizeCardRadius:
		return 12
	case sizeCardPadding:
		return 16
	case sizeSectionSpacing:
		return 22
	}
	return theme.DefaultTheme().Size(n)
}

func (appTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Monospace || s.Symbol || s.Italic {
		return theme.DefaultTheme().Font(s)
	}
	if s.Bold {
		return fontBold
	}
	return fontRegular
}

func (appTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

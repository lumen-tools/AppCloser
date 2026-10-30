//go:build gui

package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
)

//go:embed lang_en.json
var englishStrings []byte

// forceEnglish overrides the toolkit's built-in translations (dialog buttons,
// file picker) with English, so the whole UI is English on any system language.
func forceEnglish() {
	for _, l := range []string{"cs", "de", "el", "es", "fr", "ja", "pl", "pt", "pt-BR", "ru", "sv", "ta", "uk", "zh-Hans"} {
		lang.AddTranslationsForLocale(englishStrings, fyne.Locale(l))
	}
}

package gui

import (
	"fyne.io/fyne/v2/lang"

	"ai-tempo/internal/i18n"
)

var (
	T  = i18n.T
	Tf = i18n.Tf
)

// applyLanguage, ayardaki dil tercihini (boş/"auto" ise sistem dili) etkinleştirir.
func applyLanguage(pref string) {
	i18n.Set(i18n.Resolve(pref, string(lang.SystemLocale())))
}

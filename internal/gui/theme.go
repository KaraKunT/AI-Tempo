package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Uygulamaya özel renk adları (kart arka planı ve kenarlığı).
const (
	colorNameCard       fyne.ThemeColorName = "card"
	colorNameCardBorder fyne.ThemeColorName = "cardBorder"
)

var colorAccent = color.NRGBA{R: 0x7C, G: 0x5C, B: 0xFF, A: 0xFF} // mor vurgu

// modernTheme, varsayılan Fyne temasını daha yumuşak arka planlar, belirgin
// yuvarlak köşeler ve mor vurgu rengiyle modernleştirir. Açık/koyu modu izler.
type modernTheme struct{}

func (modernTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	dark := v == theme.VariantDark
	pick := func(d, l color.NRGBA) color.Color {
		if dark {
			return d
		}
		return l
	}
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		return colorAccent
	case theme.ColorNameBackground:
		return pick(color.NRGBA{R: 0x12, G: 0x12, B: 0x16, A: 0xFF}, color.NRGBA{R: 0xF2, G: 0xF2, B: 0xF7, A: 0xFF})
	case colorNameCard, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return pick(color.NRGBA{R: 0x1E, G: 0x1E, B: 0x24, A: 0xFF}, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
	case colorNameCardBorder, theme.ColorNameSeparator:
		return pick(color.NRGBA{R: 0x2C, G: 0x2C, B: 0x34, A: 0xFF}, color.NRGBA{R: 0xE3, G: 0xE3, B: 0xEA, A: 0xFF})
	case theme.ColorNameButton:
		return pick(color.NRGBA{R: 0x26, G: 0x26, B: 0x2E, A: 0xFF}, color.NRGBA{R: 0xE8, G: 0xE8, B: 0xEF, A: 0xFF})
	case theme.ColorNameHover:
		return pick(color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x12}, color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0D})
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x7C, G: 0x5C, B: 0xFF, A: 0x40}
	}
	return theme.DefaultTheme().Color(name, v)
}

func (modernTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (modernTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (modernTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 10
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return theme.DefaultTheme().Size(name)
}

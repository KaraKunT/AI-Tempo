package gui

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Version, derleme sırasında -ldflags "-X ai-tempo/internal/gui.Version=1.2.3"
// ile doldurulur (bkz. Makefile). Boşsa .app'in Info.plist sürümüne bakılır.
var Version = ""

const repoURL = "https://github.com/KaraKunT/AI-Tempo"

// appVersion, gösterilecek sürüm metnini döndürür.
func appVersion() string {
	if Version != "" {
		return Version
	}
	if v := fyne.CurrentApp().Metadata().Version; v != "" && v != "0.0.1" {
		return v
	}
	return "dev"
}

// showAbout, uygulama adı, sürümü ve bağlantılarını içeren Hakkında penceresini açar.
func showAbout(parent fyne.Window) {
	logo := canvas.NewImageFromResource(appIcon)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSquareSize(72))

	name := canvas.NewText("AI Tempo", nil)
	name.TextSize = 22
	name.TextStyle = fyne.TextStyle{Bold: true}
	name.Alignment = fyne.TextAlignCenter

	version := canvas.NewText(Tf("Sürüm %s", appVersion()), colorMuted)
	version.TextSize = 12
	version.Alignment = fyne.TextAlignCenter

	desc := widget.NewLabel(T("Claude, Cursor ve ChatGPT kullanım limitlerinizi ve temponuzu menü çubuğundan takip edin."))
	desc.Wrapping = fyne.TextWrapWord
	desc.Alignment = fyne.TextAlignCenter

	link := func(text, path string) fyne.CanvasObject {
		u, _ := url.Parse(repoURL + path)
		return container.NewCenter(widget.NewHyperlink(text, u))
	}

	copyright := canvas.NewText("© 2026 KaraKunT · "+T("MIT lisansı"), colorMuted)
	copyright.TextSize = 11
	copyright.Alignment = fyne.TextAlignCenter

	content := container.NewVBox(
		container.NewCenter(logo),
		name,
		version,
		desc,
		link("GitHub", ""),
		link(T("Sürümler ve güncellemeler"), "/releases"),
		link(T("Sorun bildir"), "/issues"),
		copyright,
	)
	d := dialog.NewCustom(T("AI Tempo Hakkında"), T("Kapat"), container.NewPadded(content), parent)
	d.Resize(fyne.NewSize(380, 0))
	d.Show()
}

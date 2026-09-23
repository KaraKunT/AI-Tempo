package gui

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed icons/claude.svg
var claudeIconBytes []byte

//go:embed icons/cursor.svg
var cursorIconBytes []byte

//go:embed icons/chatgpt.svg
var chatgptIconBytes []byte

//go:embed icons/tray.svg
var trayIconBytes []byte

//go:embed icons/app.png
var appIconBytes []byte

var (
	claudeIcon  = fyne.NewStaticResource("claude.svg", claudeIconBytes)
	cursorIcon  = fyne.NewStaticResource("cursor.svg", cursorIconBytes)
	chatgptIcon = fyne.NewStaticResource("chatgpt.svg", chatgptIconBytes)
	// ThemedResource, macOS'ta "template icon" olarak gönderilir; böylece ikon
	// koyu/açık menü çubuğuna göre otomatik beyaz/siyah görünür.
	trayIcon = theme.NewThemedResource(fyne.NewStaticResource("tray.svg", trayIconBytes))
	appIcon  = fyne.NewStaticResource("app.png", appIconBytes)
)

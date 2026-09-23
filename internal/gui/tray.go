package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"ai-tempo/internal/config"
	"ai-tempo/internal/provider"
)

// trayMenu, menü çubuğundaki (saatin yanındaki) menüyü yönetir. Her hesabın
// kota özetini menüde gösterir; bir hesaba tıklanınca ilgili sekme açılır.
// Veriler usage.Store'dan gelir; menü kendisi sorgu yapmaz.
type trayMenu struct {
	desk       desktop.App
	onShow     func(tabIndex int)
	onSettings func()
	onQuit     func()
}

func newTrayMenu(desk desktop.App, onShow func(tabIndex int), onSettings, onQuit func()) *trayMenu {
	t := &trayMenu{desk: desk, onShow: onShow, onSettings: onSettings, onQuit: onQuit}
	desk.SetSystemTrayIcon(trayIcon)
	t.rebuild()
	onStoreChange(t.rebuild)
	return t
}

func (t *trayMenu) rebuild() {
	var items []*fyne.MenuItem
	accounts := config.EnabledAccounts()
	for i, acc := range accounts {
		idx := i
		// Kota satırları bilerek aktif bırakılır: macOS devre dışı öğeleri soluk
		// gri çizdiği için okunmuyorlar. Tıklanınca ilgili sekme açılır.
		openTab := func() { t.onShow(idx) }

		header := fyne.NewMenuItem(fmt.Sprintf("%s — %s", provider.Get(acc.Provider).DisplayName(), acc.Name), openTab)
		header.Icon = providerIcon(acc.Provider)
		items = append(items, header)

		info, _ := store.Get(acc.ID)
		switch {
		case info == nil:
			items = append(items, fyne.NewMenuItem("    Yükleniyor...", openTab))
		case !info.Success:
			items = append(items, fyne.NewMenuItem("    ❌ "+info.Error, openTab))
		default:
			for _, m := range info.Metrics {
				line := fmt.Sprintf("    %s %s: %.0f%%", severityEmoji(m.Percent), m.Label, m.Percent)
				if provider.CalcPace(m.Percent, m.TimeProgress) == provider.PaceFast {
					line += " ⚠ hızlı"
				}
				if m.ResetsInfo != "" {
					line += " · ⏳ " + m.ResetsInfo
				}
				items = append(items, fyne.NewMenuItem(line, openTab))
			}
			if rc := info.ResetCredits; rc != nil && rc.Error == "" {
				line := fmt.Sprintf("    🎟 Sıfırlama hakkı: %d", len(rc.Available))
				if len(rc.Available) > 0 {
					line += " · bitiş " + provider.ShortDateTime(rc.Available[0].ExpiresAt)
				}
				items = append(items, fyne.NewMenuItem(line, openTab))
			}
		}
		items = append(items, fyne.NewMenuItemSeparator())
	}
	if len(accounts) == 0 {
		items = append(items, fyne.NewMenuItem("Henüz hesap yok — Ayarlar'dan ekleyin", t.onSettings), fyne.NewMenuItemSeparator())
	}

	loading, updated := store.Status()
	refreshLabel := "Tümünü Yenile"
	if loading {
		refreshLabel = "Yenileniyor..."
	} else if !updated.IsZero() {
		refreshLabel = fmt.Sprintf("Tümünü Yenile (son: %s)", updated.Format("15:04"))
	}
	refreshItem := fyne.NewMenuItem(refreshLabel, func() { store.Refresh(config.EnabledAccounts(), true) })
	refreshItem.Disabled = loading

	items = append(items,
		refreshItem,
		fyne.NewMenuItem("Pencereyi Göster", func() { t.onShow(-1) }),
		fyne.NewMenuItem("Ayarlar…", t.onSettings),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Çıkış", t.onQuit),
	)
	t.desk.SetSystemTrayMenu(fyne.NewMenu("AI Tempo", items...))
}

func severityEmoji(percent float64) string {
	switch {
	case percent >= 100:
		return "🔴"
	case percent >= 80:
		return "🟠"
	default:
		return "🟢"
	}
}

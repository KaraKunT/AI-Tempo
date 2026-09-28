// Package gui, AI Tempo'nun menü çubuğu menüsü, ana penceresi ve ayarlar
// penceresini içerir.
package gui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"ai-tempo/internal/config"
	"ai-tempo/internal/history"
	"ai-tempo/internal/provider"
	"ai-tempo/internal/usage"
)

const (
	prefWindowWidth  = "window_width"
	prefWindowHeight = "window_height"
	defaultWidth     = 560
	defaultHeight    = 720
)

// store, pencere ve menü çubuğunun paylaştığı sorgu deposudur.
var store = usage.Default

// onStoreChange, sorgu sonuçları değiştiğinde f'yi UI iş parçacığında çalıştırır.
func onStoreChange(f func()) { store.OnChange(func() { fyne.Do(f) }) }

var (
	// accountsChanged, hesaplar veya ayarlar değişince pencereyi ve sorguları yeniler.
	accountsChanged = func() {}
	mainWindow      fyne.Window
	tabContainer    *container.AppTabs
	accountTabs     []*accountTabHandle
	mainBody        *fyne.Container
)

// Run, uygulamayı başlatır ve kapanana kadar bloklar.
func Run() {
	// Başka bir kopya çalışıyorsa onun penceresini öne getir ve çık.
	if !claimSingleInstance(func() {
		fyne.Do(func() {
			if mainWindow != nil {
				mainWindow.Show()
				activateApp()
				mainWindow.RequestFocus()
			}
		})
	}) {
		fmt.Println("AI Tempo zaten çalışıyor; mevcut pencere öne getirildi.")
		return
	}
	defer releaseSingleInstance()

	var err error
	config.Current, err = config.Load()
	if err != nil {
		fmt.Println("❌ Hata: ayarlar okunamadı:", err)
		os.Exit(1)
	}

	if err := history.Open(config.DataDir()); err != nil {
		fmt.Println("⚠ Sorgu geçmişi açılamadı:", err)
	}
	history.SetRetention(config.Current.HistoryDays)
	applyLanguage(config.Current.Language)

	myApp := app.NewWithID("com.karakunt.ai-tempo")
	myApp.SetIcon(appIcon)
	myApp.Settings().SetTheme(modernTheme{})
	prefs := myApp.Preferences()

	mainWindow = myApp.NewWindow("")
	mainWindow.SetTitle("AI Tempo")

	width := prefs.FloatWithFallback(prefWindowWidth, defaultWidth)
	height := prefs.FloatWithFallback(prefWindowHeight, defaultHeight)
	mainWindow.Resize(fyne.NewSize(float32(width), float32(height)))

	openSettings := func() {}
	accountsChanged = func() {
		loadAccountTabs(openSettings)
		// Dil değişmiş olabilir: başlık ve düğmeler yeniden oluşturulur.
		mainWindow.SetContent(buildMainLayout(mainBody, newStatusBar(), refreshCurrentTab, refreshAllTabs, openSettings))
		store.Notify()
		store.Refresh(config.EnabledAccounts(), false) // yeni/değişen hesaplar hemen sorgulanır
	}
	openSettings = func() { showSettings(myApp, accountsChanged) }

	tabContainer = container.NewAppTabs()
	mainBody = container.NewStack()
	loadAccountTabs(openSettings)
	tabContainer.OnSelected = func(*container.TabItem) { updateStatus() }
	onStoreChange(func() {
		for _, h := range accountTabs {
			h.render()
		}
		tabContainer.Refresh()
	})

	onStoreChange(updateStatus)
	updateStatus()
	mainWindow.SetContent(buildMainLayout(mainBody, newStatusBar(), refreshCurrentTab, refreshAllTabs, openSettings))

	saveWindowSize := func() {
		size := mainWindow.Canvas().Size()
		prefs.SetFloat(prefWindowWidth, float64(size.Width))
		prefs.SetFloat(prefWindowHeight, float64(size.Height))
	}

	// Menü çubuğuna (saatin yanına) kota özetli bir menü ekle. Pencereyi
	// kapatmak yerine gizler, uygulama arka planda çalışmaya devam eder ve
	// menüden tekrar açılabilir.
	if desk, ok := myApp.(desktop.App); ok {
		newTrayMenu(desk, func(tabIndex int) {
			if tabIndex >= 0 && tabIndex < len(tabContainer.Items) {
				tabContainer.SelectIndex(tabIndex)
			}
			mainWindow.Show()
			activateApp()
			mainWindow.RequestFocus()
		}, openSettings, func() {
			mainWindow.Show()
			activateApp()
			showAbout(mainWindow)
		}, func() {
			saveWindowSize()
			myApp.Quit()
		})

		mainWindow.SetCloseIntercept(func() {
			saveWindowSize()
			mainWindow.Hide()
		})
	} else {
		// Masaüstü sürücüsü değilse (beklenmeyen durum) eski davranışa dön.
		mainWindow.SetCloseIntercept(func() {
			saveWindowSize()
			mainWindow.Close()
		})
	}

	myApp.Lifecycle().SetOnStarted(func() {
		fyne.Do(func() {
			applyDockVisibility()
			rememberWindowFrame(mainWindow, "AITempoMainWindow")
		})
	})
	store.Start()
	if len(config.Current.Accounts) == 0 {
		openSettings()
	}
	if config.LegacyConfigPath != "" {
		dialog.ShowInformation(T("Hesaplar içe aktarıldı"),
			T("config.json'daki hesaplar Ayarlar'a, anahtarlar Keychain'e taşındı.\nArtık bu dosya kullanılmıyor; anahtarlar düz metin olarak durmasın diye silebilirsiniz:")+"\n\n"+config.LegacyConfigPath,
			mainWindow)
	}

	mainWindow.ShowAndRun()
}

// loadAccountTabs, etkin hesaplar için sekmeleri (yeniden) oluşturur; seçili
// sekmeyi mümkünse korur. Hiç hesap yoksa karşılama ekranını gösterir.
func loadAccountTabs(onSettings func()) {
	selectedID := ""
	if sel := tabContainer.Selected(); sel != nil {
		for _, h := range accountTabs {
			if h.tabItem == sel {
				selectedID = h.account.ID
			}
		}
	}

	accountTabs = nil
	var items []*container.TabItem
	for _, account := range config.EnabledAccounts() {
		handle := createAccountTab(account)
		items = append(items, handle.tabItem)
		accountTabs = append(accountTabs, handle)
	}
	tabContainer.SetItems(items)
	for i, h := range accountTabs {
		if h.account.ID == selectedID {
			tabContainer.SelectIndex(i)
		}
	}

	if len(accountTabs) == 0 {
		mainBody.Objects = []fyne.CanvasObject{newEmptyState(onSettings)}
	} else {
		mainBody.Objects = []fyne.CanvasObject{tabContainer}
	}
	mainBody.Refresh()
}

// refreshCurrentTab, yalnızca o an ekranda görünen sekmenin hesabını yeniden sorgular.
func refreshCurrentTab() {
	sel := tabContainer.Selected()
	for _, h := range accountTabs {
		if h.tabItem == sel {
			if store.Refresh([]config.Account{h.account}, true) == 0 {
				flashStatus(Tf("“%s” az önce güncellendi (30 sn içinde tekrar sorgulanmaz)", h.account.Name))
			}
			return
		}
	}
}

// refreshAllTabs, tüm etkin hesapları yeniden sorgular.
func refreshAllTabs() {
	if store.Refresh(config.EnabledAccounts(), true) == 0 {
		flashStatus(T("Tüm hesaplar az önce güncellendi (30 sn içinde tekrar sorgulanmaz)"))
	}
}

var (
	statusText    *canvas.Text
	statusSpinner *widget.Activity
	flashUntil    time.Time
)

// newStatusBar, başlığın altındaki "Yenileniyor… / Son güncelleme" satırını oluşturur.
func newStatusBar() fyne.CanvasObject {
	return container.NewCenter(container.NewHBox(statusSpinner, statusText))
}

// selectedHandle, o an seçili sekmenin hesabını döndürür.
func selectedHandle() *accountTabHandle {
	sel := tabContainer.Selected()
	for _, h := range accountTabs {
		if h.tabItem == sel {
			return h
		}
	}
	return nil
}

// updateStatus, durum satırında yalnızca seçili sekmenin hesabının durumunu gösterir.
func updateStatus() {
	if statusText == nil {
		statusText = canvas.NewText("", colorMuted)
		statusText.TextSize = 12
		statusSpinner = widget.NewActivity()
	}
	h := selectedHandle()
	loading := false
	if h != nil {
		var info *provider.RateLimitInfo
		info, loading = store.Get(h.account.ID)
		if !loading && time.Now().Before(flashUntil) {
			return
		}
		name := h.account.Name
		last := store.LastFetched(h.account.ID)
		switch {
		case loading:
			statusText.Text, statusText.Color = name+" · "+T("Yenileniyor…"), colorAccent
		case info == nil:
			statusText.Text, statusText.Color = name+" · "+T("Henüz sorgulanmadı"), colorMuted
		case info.Success:
			statusText.Text, statusText.Color = "✓ "+name+" · "+T("Son güncelleme:")+" "+last.Format("15:04:05"), colorMuted
		default:
			statusText.Text, statusText.Color = "✗ "+name+" · "+T("Hata")+" · "+last.Format("15:04:05"), colorDanger
		}
	} else {
		statusText.Text = ""
	}
	if loading {
		statusSpinner.Show()
		statusSpinner.Start()
	} else {
		statusSpinner.Stop()
		statusSpinner.Hide()
	}
	statusText.Refresh()
}

// flashStatus, durum satırında birkaç saniyeliğine bir bilgi mesajı gösterir.
func flashStatus(msg string) {
	flashUntil = time.Now().Add(3 * time.Second)
	statusText.Text = "ℹ " + msg
	statusText.Color = colorWarning
	statusText.Refresh()
	time.AfterFunc(3*time.Second, func() { fyne.Do(updateStatus) })
}

// applyDockVisibility, ayara göre uygulamayı Dock'ta gösterir veya gizler.
func applyDockVisibility() {
	if config.Current.ShowInDock {
		showInDock()
	} else {
		hideFromDock()
	}
}

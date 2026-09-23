// Package gui, AI Tempo'nun menü çubuğu menüsü, ana penceresi ve ayarlar
// penceresini içerir.
package gui

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"

	"ai-tempo/internal/config"
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
	mainWindow   fyne.Window
	tabContainer *container.AppTabs
	accountTabs  []*accountTabHandle
	mainBody     *fyne.Container
)

// Run, uygulamayı başlatır ve kapanana kadar bloklar.
func Run() {
	var err error
	config.Current, err = config.Load()
	if err != nil {
		fmt.Println("❌ Hata: ayarlar okunamadı:", err)
		os.Exit(1)
	}

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
	openSettings = func() {
		showSettings(myApp, func() {
			loadAccountTabs(openSettings)
			store.Notify()
			store.Refresh(config.EnabledAccounts(), false) // yeni/değişen hesaplar hemen sorgulanır
		})
	}

	tabContainer = container.NewAppTabs()
	mainBody = container.NewStack()
	loadAccountTabs(openSettings)
	onStoreChange(func() {
		for _, h := range accountTabs {
			h.render()
		}
	})

	mainWindow.SetContent(buildMainLayout(mainBody, refreshCurrentTab, refreshAllTabs, openSettings))

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

	myApp.Lifecycle().SetOnStarted(func() { fyne.Do(hideFromDock) })
	store.Start()
	if len(config.Current.Accounts) == 0 {
		openSettings()
	}
	if config.LegacyConfigPath != "" {
		dialog.ShowInformation("Hesaplar içe aktarıldı",
			"config.json'daki hesaplar Ayarlar'a, anahtarlar Keychain'e taşındı.\n"+
				"Artık bu dosya kullanılmıyor; anahtarlar düz metin olarak durmasın diye silebilirsiniz:\n\n"+config.LegacyConfigPath,
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
			store.Refresh([]config.Account{h.account}, true)
			return
		}
	}
}

// refreshAllTabs, tüm etkin hesapları yeniden sorgular.
func refreshAllTabs() {
	store.Refresh(config.EnabledAccounts(), true)
}

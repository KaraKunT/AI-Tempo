package gui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"ai-tempo/internal/config"
	"ai-tempo/internal/provider"
	"ai-tempo/internal/usage"
)

// Renk paleti
var (
	colorGood    = color.NRGBA{R: 0x34, G: 0xC7, B: 0x59, A: 0xFF} // yeşil
	colorWarning = color.NRGBA{R: 0xFF, G: 0x9F, B: 0x0A, A: 0xFF} // turuncu
	colorDanger  = color.NRGBA{R: 0xFF, G: 0x45, B: 0x3A, A: 0xFF} // kırmızı
	colorMuted   = color.NRGBA{R: 0x8E, G: 0x8E, B: 0x93, A: 0xFF} // gri
)

// severityColor, kullanım yüzdesine göre uygun rengi döndürür.
func severityColor(percent float64) color.Color {
	switch {
	case percent >= 100:
		return colorDanger
	case percent >= 80:
		return colorWarning
	default:
		return colorGood
	}
}

// buildMainLayout, ana pencerenin header + tab yapısını oluşturur.
// onRefreshCurrent yalnızca o an seçili sekmeyi, onRefreshAll ise tüm sekmeleri yeniler.
// status, başlığın altında gösterilen yenileme durum satırıdır.
func buildMainLayout(body, status fyne.CanvasObject, onRefreshCurrent, onRefreshAll, onSettings func()) fyne.CanvasObject {
	logo := canvas.NewImageFromResource(appIcon)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSquareSize(40))
	titleIcon := container.NewCenter(logo)

	titleText := canvas.NewText("AI Tempo", nil)
	titleText.TextSize = 20
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	subtitleText := canvas.NewText(T("Kullanım Limitleri Panosu"), colorMuted)
	subtitleText.TextSize = 13

	titleBox := container.NewVBox(titleText, subtitleText)

	refreshCurrentBtn := widget.NewButtonWithIcon(T("Yenile"), theme.ViewRefreshIcon(), onRefreshCurrent)
	refreshCurrentBtn.Importance = widget.HighImportance

	refreshAllBtn := widget.NewButtonWithIcon(T("Tümünü Yenile"), theme.ViewRestoreIcon(), onRefreshAll)
	refreshAllBtn.Importance = widget.LowImportance

	configBtn := widget.NewButtonWithIcon(T("Ayarlar"), theme.SettingsIcon(), onSettings)
	configBtn.Importance = widget.LowImportance

	buttons := container.NewCenter(container.NewHBox(configBtn, refreshAllBtn, refreshCurrentBtn))

	header := container.NewBorder(
		nil, nil,
		container.NewHBox(titleIcon, titleBox),
		buttons,
	)

	headerPadded := container.NewPadded(container.NewPadded(header))

	return container.NewBorder(
		container.NewVBox(headerPadded, status),
		nil, nil, nil,
		container.NewPadded(body),
	)
}

// providerIcon, servise göre tab ikonunu (marka renginde basit bir logo) döndürür.
func providerIcon(providerID string) fyne.Resource {
	switch providerID {
	case "claude":
		return claudeIcon
	case "cursor":
		return cursorIcon
	case "chatgpt":
		return chatgptIcon
	default:
		return theme.AccountIcon()
	}
}

// accountTabHandle, bir hesap sekmesini ve içeriğini usage.Store'daki son
// sonuca göre yeniden çizen fonksiyonu bir arada tutar.
type accountTabHandle struct {
	account config.Account
	tabItem *container.TabItem
	render  func()
}

// createAccountTab, tek bir hesap için sekme oluşturur. Sorguyu kendisi yapmaz;
// usage.Store'daki sonucu gösterir (bkz. internal/usage).
func createAccountTab(account config.Account) *accountTabHandle {
	prov := provider.Get(account.Provider)

	body := container.NewVBox()
	logSection, refreshLog := newLogSection(account.ID, func() map[string]string {
		labels := map[string]string{}
		if info, _ := store.Get(account.ID); info != nil {
			for _, m := range info.Metrics {
				labels[m.ID] = m.Label
			}
		}
		return labels
	})
	chartCards := container.NewVBox()
	chartRange := rangePeriod
	var lastInfo *provider.RateLimitInfo
	renderCharts := func() {
		chartCards.Objects = newChartsSection(account.ID, lastInfo, chartRange)
		chartCards.Refresh()
	}
	chartFilter := newRangeSelector([]timeRange{rangePeriod, rangeToday, rangeYesterday, rangeWeek, rangeMonth}, chartRange, func(r timeRange) {
		chartRange = r
		renderCharts()
	})
	charts := container.NewVBox(container.NewCenter(chartFilter), chartCards)
	charts.Hide()
	scrollContent := container.NewScroll(container.NewPadded(container.NewVBox(body, charts, logSection)))
	var tabItem *container.TabItem

	render := func() {
		info, loading := store.Get(account.ID)
		body.RemoveAll()
		switch {
		case info == nil:
			body.Add(newUsageCard(T("Yükleniyor..."), prov.DisplayName()).container)
		case !info.Success || len(info.Metrics) == 0:
			errCard := newUsageCard(prov.DisplayName(), account.Name)
			errCard.setError(info.Error)
			body.Add(errCard.container)
			body.Add(newRetryCounterRow(account, info.AuthExpired))
		default:
			for _, m := range info.Metrics {
				c := newUsageCard(m.Label, m.Subtitle)
				c.update(m.Percent, m.ResetsInfo, m.TimeProgress)
				body.Add(c.container)
			}
			if info.ResetCredits != nil {
				body.Add(newResetCreditsCard(account, info.ResetCredits))
			}
		}
		if loading && info != nil {
			bar := widget.NewProgressBarInfinite()
			body.Objects = append([]fyne.CanvasObject{container.NewPadded(bar)}, body.Objects...)
		}
		body.Refresh()
		refreshLog()
		if info != nil && info.Success {
			lastInfo = info
			renderCharts()
			charts.Show()
		}
		if tabItem != nil {
			tabItem.Text = tabTitle(account.Name, info, loading)
		}
	}
	tabItem = container.NewTabItemWithIcon(account.Name, providerIcon(account.Provider), scrollContent)
	render()

	return &accountTabHandle{
		account: account,
		tabItem: tabItem,
		render:  render,
	}
}

// newRetryCounterRow, hatalı hesap için otomatik deneme sayacını ve sayacı
// sıfırlayıp hemen yeniden deneyen düğmeyi gösterir.
// authExpired ise yanına hesabın düzenleme penceresini açan "Anahtarı Güncelle" eklenir.
func newRetryCounterRow(account config.Account, authExpired bool) fyne.CanvasObject {
	n := store.Failures(account.ID)
	msg := Tf("Deneme: %d/%d", n, usage.MaxAutoRetries)
	col := colorMuted
	if n >= usage.MaxAutoRetries {
		msg += " — " + T("otomatik kontrol durduruldu")
		col = colorDanger
	}
	text := canvas.NewText(msg, col)
	text.TextSize = 12
	btn := widget.NewButtonWithIcon(T("Sayacı Sıfırla"), theme.ViewRefreshIcon(), func() {
		store.ResetFailures(account)
	})
	btn.Importance = widget.LowImportance
	buttons := container.NewHBox(btn)
	if authExpired {
		update := widget.NewButtonWithIcon(T("Anahtarı Güncelle"), theme.DocumentCreateIcon(), func() {
			editAccountFromMain(account.ID)
		})
		update.Importance = widget.HighImportance
		buttons.Objects = []fyne.CanvasObject{update, btn}
	}
	return container.NewPadded(container.NewBorder(nil, nil, nil, buttons, container.NewCenter(text)))
}

// tabTitle, sekme başlığına hesabın durumunu ekler; "Tümünü Yenile"de hangi
// hesabın sorgulandığı ve hangisinin hata verdiği sekmelerden görünür.
func tabTitle(name string, info *provider.RateLimitInfo, loading bool) string {
	switch {
	case loading:
		return "⟳ " + name
	case info != nil && !info.Success:
		return "⚠ " + name
	}
	return name
}

// newEmptyState, hiç hesap yokken gösterilen karşılama içeriğidir.
func newEmptyState(onSettings func()) fyne.CanvasObject {
	title := canvas.NewText(T("Henüz hesap eklenmedi"), nil)
	title.TextSize = 18
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter
	sub := canvas.NewText(T("Claude, Cursor veya ChatGPT hesabınızı Ayarlar'dan ekleyin."), colorMuted)
	sub.Alignment = fyne.TextAlignCenter
	btn := widget.NewButtonWithIcon(T("Hesap Ekle"), theme.ContentAddIcon(), onSettings)
	btn.Importance = widget.HighImportance
	return container.NewCenter(container.NewVBox(title, sub, container.NewCenter(btn)))
}

// usageCard, bir kota kartının (mevcut oturum / haftalık) canlı güncellenen bileşenlerini tutar.
type usageCard struct {
	container   *fyne.Container
	title       *canvas.Text
	percentText *canvas.Text
	bar         *coloredProgressBar
	resetLabel  *widget.Label
	statusText  *canvas.Text
	statusPill  *canvas.Rectangle
	paceText    *canvas.Text
}

func newUsageCard(title, subtitle string) *usageCard {
	titleText := canvas.NewText(title, nil)
	titleText.TextSize = 17
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	subtitleText := canvas.NewText(subtitle, colorMuted)
	subtitleText.TextSize = 12

	percentText := canvas.NewText("...", colorMuted)
	percentText.TextSize = 34
	percentText.TextStyle = fyne.TextStyle{Bold: true}
	percentText.Alignment = fyne.TextAlignTrailing

	statusText := canvas.NewText(T("Yükleniyor"), colorMuted)
	statusText.TextSize = 11
	statusText.TextStyle = fyne.TextStyle{Bold: true}
	statusText.Alignment = fyne.TextAlignCenter

	// Durum "hap" rozeti: yarı saydam renkli yuvarlak zemin üzerinde metin.
	statusPill := canvas.NewRectangle(withAlpha(colorMuted, 0x2E))
	statusPill.CornerRadius = 10
	badge := container.NewStack(statusPill, container.New(layout.NewCustomPaddedLayout(3, 3, 10, 10), statusText))

	headerRow := container.NewBorder(
		nil, nil,
		container.NewVBox(titleText, subtitleText, container.NewHBox(badge)),
		percentText,
	)

	bar := newColoredProgressBar(12)

	resetLabel := widget.NewLabel("")
	resetLabel.Wrapping = fyne.TextWrapWord

	// Tempo: kullanım çubuğundaki mor işaretle (döneme göre geçen süre) kıyas.
	paceText := canvas.NewText("", colorMuted)
	paceText.TextSize = 12
	paceText.TextStyle = fyne.TextStyle{Bold: true}
	paceText.Alignment = fyne.TextAlignTrailing

	content := container.NewVBox(
		headerRow,
		container.New(layout.NewCustomPaddedLayout(12, 8, 0, 0), bar),
		container.NewBorder(nil, nil, nil, paceText, resetLabel),
	)

	return &usageCard{
		container:   newRoundedCard(content),
		title:       titleText,
		percentText: percentText,
		bar:         bar,
		resetLabel:  resetLabel,
		statusText:  statusText,
		statusPill:  statusPill,
		paceText:    paceText,
	}
}

func (c *usageCard) update(percent float64, resetInfo string, timeProgress float64) {
	col := severityColor(percent)

	c.percentText.Text = fmt.Sprintf("%.0f%%", percent)
	c.percentText.Color = col
	c.percentText.Refresh()

	c.bar.SetValue(percent/100, col)

	switch {
	case percent >= 100:
		c.statusText.Text = "● " + T("DOLU")
	case percent >= 80:
		c.statusText.Text = "● " + T("YÜKSEK")
	case percent <= 0:
		c.statusText.Text = "● " + T("SIFIRLANDI")
	default:
		c.statusText.Text = "● " + T("NORMAL")
	}
	c.setStatusColor(col)

	if resetInfo != "" {
		c.resetLabel.SetText(T("Sıfırlanmaya:") + " " + resetInfo)
	} else {
		c.resetLabel.SetText("")
	}

	c.bar.SetMarker(timeProgress)
	label, pc := paceLabel(provider.CalcPace(percent, timeProgress))
	c.paceText.Text = label
	c.paceText.Color = pc
	c.paceText.Refresh()
}

// paceLabel, tempo için arayüzde gösterilecek metni ve rengi döndürür.
func paceLabel(p provider.Pace) (string, color.Color) {
	switch p {
	case provider.PaceFast:
		return T("Tempo: hızlı") + " ⚠", colorWarning
	case provider.PaceSlow:
		return T("Tempo: rahat"), colorGood
	case provider.PaceNormal:
		return T("Tempo: normal"), colorAccent
	}
	return "", colorMuted
}

func (c *usageCard) setError(msg string) {
	c.percentText.Text = "—"
	c.percentText.Color = colorDanger
	c.percentText.Refresh()

	c.statusText.Text = "● " + T("HATA")
	c.setStatusColor(colorDanger)

	c.bar.SetValue(0, colorMuted)
	c.bar.SetMarker(-1)
	c.paceText.Text = ""
	c.paceText.Refresh()
	c.resetLabel.SetText(msg)
}

func (c *usageCard) setStatusColor(col color.Color) {
	c.statusText.Color = col
	c.statusText.Refresh()
	c.statusPill.FillColor = withAlpha(col, 0x2E)
	c.statusPill.Refresh()
}

// newRoundedCard, içeriği ince kenarlıklı, yuvarlak köşeli bir kart zeminine yerleştirir.
func newRoundedCard(content fyne.CanvasObject) *fyne.Container {
	bg := canvas.NewRectangle(theme.Color(colorNameCard))
	bg.CornerRadius = 16
	bg.StrokeColor = theme.Color(colorNameCardBorder)
	bg.StrokeWidth = 1
	inner := container.New(layout.NewCustomPaddedLayout(16, 16, 18, 18), content)
	return container.New(layout.NewCustomPaddedLayout(6, 6, 4, 4), container.NewStack(bg, inner))
}

// withAlpha, bir rengin saydamlığını değiştirir.
func withAlpha(c color.Color, a uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: a}
}

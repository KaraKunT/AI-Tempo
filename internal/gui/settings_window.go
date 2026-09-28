package gui

import (
	"context"
	"fmt"
	"image/color"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"ai-tempo/internal/config"
	"ai-tempo/internal/history"
	"ai-tempo/internal/i18n"
	"ai-tempo/internal/provider"
)

var refreshOptions = []int{5, 10, 15, 30, 60, 120, 240, 480, 960, 1440}

// providerHelp, hesap düzenleyicide anahtarın nereden alınacağını kısaca anlatır
// (ayrıntılı adımlar README'de).
var providerHelp = map[string]struct{ key, org string }{
	"claude": {
		key: "claude.ai → Geliştirici Araçları → Network → …/usage isteği → Cookie: sessionKeyV3",
		org: "Aynı isteğin URL'sinde /organizations/ ile /usage arasındaki UUID",
	},
	"cursor": {
		key: "cursor.com/dashboard/spending → get-current-period-usage isteği → Cookie: WorkosCursorSessionToken (%3A%3A → ::)",
		org: "Cursor için gerekmez",
	},
	"chatgpt": {
		key: "chatgpt.com → backend-api/wham/usage isteği → authorization: Bearer <token> (sadece token)",
		org: "Aynı istekteki chatgpt-account-id başlığı",
	},
}

// consoleSnippets, her sağlayıcının sitesinde tarayıcı konsoluna yapıştırıldığında
// anahtarı/ID'yi tam haliyle yazdıran kodlardır. HttpOnly cookie'ler JavaScript'ten
// okunamaz; o durumda kod, değerin nereden alınacağını yazar.
var consoleSnippets = map[string]struct{ site, code string }{
	"chatgpt": {"chatgpt.com", `fetch('/api/auth/session').then(r => r.json()).then(s => {
  const id = s.account?.id || JSON.parse(atob(s.accessToken.split('.')[1]))['https://api.openai.com/auth'].chatgpt_account_id;
  console.log('TOKEN:\n' + s.accessToken + '\n\nACCOUNT ID:\n' + id + '\n\nExpires: ' + new Date(s.expires).toLocaleString());
  copy(s.accessToken);
});`},
	"claude": {"claude.ai", `fetch('/api/organizations').then(r => r.json()).then(orgs => {
  orgs.forEach(o => console.log('ORGANIZATION ID (' + o.name + '):\n' + o.uuid));
  const m = document.cookie.match(/(?:^|; )sessionKeyV3=([^;]+)/);
  if (m) { console.log('SESSION KEY:\n' + m[1]); copy(m[1]); }
  else console.log('sessionKeyV3 is HttpOnly; copy it from Application → Cookies → claude.ai');
});`},
	"cursor": {"cursor.com", `(() => {
  const m = document.cookie.match(/(?:^|; )WorkosCursorSessionToken=([^;]+)/);
  if (m) { const t = decodeURIComponent(m[1]); console.log('TOKEN:\n' + t); copy(t); }
  else console.log('WorkosCursorSessionToken is HttpOnly; copy it from Application → Cookies → cursor.com');
})();`},
}

// showConsoleSnippet, sağlayıcının konsol kodunu kopyalanabilir bir metin alanında gösterir.
func showConsoleSnippet(parent fyne.Window, providerID string) {
	sn, ok := consoleSnippets[providerID]
	if !ok {
		return
	}
	code := widget.NewMultiLineEntry()
	code.SetText(sn.code)
	code.Wrapping = fyne.TextWrapBreak
	code.SetMinRowsVisible(7)
	copyBtn := widget.NewButtonWithIcon(T("Panoya Kopyala"), theme.ContentCopyIcon(), func() {
		fyne.CurrentApp().Clipboard().SetContent(sn.code)
	})
	copyBtn.Importance = widget.HighImportance
	content := container.NewVBox(
		hintText(Tf("%s açıkken Geliştirici Araçları → Console sekmesine yapıştırıp Enter'a basın. Değerler tam olarak yazılır, anahtar okunabiliyorsa panoya da kopyalanır.", sn.site)),
		code,
		container.NewCenter(copyBtn),
	)
	d := dialog.NewCustom(provider.Get(providerID).DisplayName()+" "+T("Konsol Kodu"), T("Kapat"), content, parent)
	d.Resize(fyne.NewSize(560, 400))
	d.Show()
}

var (
	settingsWindow  fyne.Window
	settingsRebuild func() // Ayarlar açıkken hesap listesini yeniler
)

// applyAccountEdit, düzenlenen hesabı (ve girildiyse yeni anahtarı) kaydedilmek
// üzere uygular; ayar dosyasını yazmaz. Keychain hatasında false döner.
func applyAccountEdit(parent fyne.Window, updated config.Account, newKey string) bool {
	if newKey != "" {
		if err := config.KeychainSet(updated.ID, newKey); err != nil {
			dialog.ShowError(fmt.Errorf("%s: %w", T("Anahtar Keychain'e kaydedilemedi"), err), parent)
			return false
		}
		updated.SessionKey = newKey
	}
	for i := range config.Current.Accounts {
		if config.Current.Accounts[i].ID == updated.ID {
			config.Current.Accounts[i] = updated
		}
	}
	store.Forget(updated.ID)
	return true
}

// editAccountFromMain, ana penceredeki "Anahtarı Güncelle" düğmesi için hesabın
// düzenleme penceresini açar; kaydedilince hesap hemen yeniden sorgulanır.
func editAccountFromMain(accountID string) {
	for i := range config.Current.Accounts {
		if config.Current.Accounts[i].ID != accountID {
			continue
		}
		showAccountEditor(mainWindow, &config.Current.Accounts[i], func(updated config.Account, newKey string) {
			if !applyAccountEdit(mainWindow, updated, newKey) {
				return
			}
			if err := config.Current.Save(); err != nil {
				dialog.ShowError(fmt.Errorf("%s: %w", T("Ayarlar kaydedilemedi"), err), mainWindow)
				return
			}
			if settingsRebuild != nil {
				settingsRebuild()
			}
			accountsChanged()
		})
		return
	}
}

// showSettings, ayarlar penceresini açar (zaten açıksa öne getirir).
// onChanged, hesaplar veya ayarlar değiştiğinde çağrılır.
func showSettings(app fyne.App, onChanged func()) {
	if settingsWindow != nil {
		settingsWindow.Show()
		activateApp()
		settingsWindow.RequestFocus()
		return
	}
	w := app.NewWindow(T("Ayarlar"))
	settingsWindow = w
	w.SetOnClosed(func() {
		settingsWindow = nil
		settingsRebuild = nil
	})
	w.Resize(fyne.NewSize(760, 720))

	accountList := container.NewVBox()
	var rebuild func()

	save := func() {
		if err := config.Current.Save(); err != nil {
			dialog.ShowError(fmt.Errorf("%s: %w", T("Ayarlar kaydedilemedi"), err), w)
			return
		}
		onChanged()
		rebuild()
	}

	rebuild = func() {
		accountList.RemoveAll()
		if len(config.Current.Accounts) == 0 {
			empty := canvas.NewText(T("Henüz hesap yok."), colorMuted)
			accountList.Add(container.NewPadded(empty))
		}
		for i := range config.Current.Accounts {
			idx := i
			acc := config.Current.Accounts[i]

			icon := widget.NewIcon(providerIcon(acc.Provider))
			name := canvas.NewText(acc.Name, nil)
			name.TextStyle = fyne.TextStyle{Bold: true}
			detail := provider.Get(acc.Provider).DisplayName()
			if acc.SessionKey == "" {
				detail += " · ⚠ " + T("anahtar yok")
			}
			sub := canvas.NewText(detail, colorMuted)
			sub.TextSize = 11

			enabled := widget.NewCheck(T("Etkin"), func(on bool) {
				if config.Current.Accounts[idx].Enabled == on {
					return
				}
				config.Current.Accounts[idx].Enabled = on
				save()
			})
			enabled.SetChecked(acc.Enabled)

			edit := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
				showAccountEditor(w, &config.Current.Accounts[idx], func(updated config.Account, newKey string) {
					if applyAccountEdit(w, updated, newKey) {
						save()
					}
				})
			})
			edit.Importance = widget.LowImportance

			del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				dialog.ShowConfirm(T("Hesabı sil"), Tf("“%s” silinsin mi? Anahtarı da Keychain'den kaldırılır.", acc.Name), func(ok bool) {
					if !ok {
						return
					}
					config.KeychainDelete(acc.ID)
					history.Delete(acc.ID)
					store.Forget(acc.ID)
					config.Current.Accounts = append(config.Current.Accounts[:idx], config.Current.Accounts[idx+1:]...)
					save()
				}, w)
			})
			del.Importance = widget.LowImportance

			row := container.NewBorder(nil, nil,
				container.NewHBox(icon, container.NewVBox(name, sub)),
				container.NewHBox(enabled, edit, del),
			)
			accountList.Add(newRoundedCard(row))
		}
		accountList.Refresh()
	}
	rebuild()
	settingsRebuild = rebuild

	addBtn := widget.NewButtonWithIcon(T("Hesap Ekle"), theme.ContentAddIcon(), func() {
		showAccountEditor(w, nil, func(acc config.Account, newKey string) {
			acc.ID = config.NewAccountID()
			if err := config.KeychainSet(acc.ID, newKey); err != nil {
				dialog.ShowError(fmt.Errorf("%s: %w", T("Anahtar Keychain'e kaydedilemedi"), err), w)
				return
			}
			acc.SessionKey = newKey
			config.Current.Accounts = append(config.Current.Accounts, acc)
			save()
		})
	})
	addBtn.Importance = widget.HighImportance

	labels := make([]string, len(refreshOptions))
	for i, m := range refreshOptions {
		labels[i] = refreshLabel(m)
	}
	refreshSelect := widget.NewSelect(labels, func(sel string) {
		for i, l := range labels {
			if l == sel && config.Current.RefreshMinutes != refreshOptions[i] {
				config.Current.RefreshMinutes = refreshOptions[i]
				save()
			}
		}
	})
	refreshSelect.SetSelected(refreshLabel(config.Current.RefreshMinutes))

	langPref := config.Current.Language
	if langPref == "" {
		langPref = i18n.Auto
	}
	langNames := make([]string, len(i18n.Languages))
	for i, l := range i18n.Languages {
		langNames[i] = T(l.Name)
	}
	langSelect := widget.NewSelect(langNames, nil)
	for i, l := range i18n.Languages {
		if l.Code == langPref {
			langSelect.SetSelected(langNames[i])
		}
	}
	langSelect.OnChanged = func(sel string) {
		for i, l := range i18n.Languages {
			if langNames[i] == sel && l.Code != langPref {
				applyLanguage(l.Code)
				config.Current.Language = l.Code
				// Sonuçlardaki metinler (gösterge adları, hatalar) yeni dilde gelsin diye yeniden sorgulanır.
				for _, a := range config.Current.Accounts {
					store.Forget(a.ID)
				}
				save()
				// Pencere yeni dille yeniden açılır.
				w.Close()
				showSettings(app, onChanged)
			}
		}
	}

	dockCheck := widget.NewCheck(T("Dock'ta göster"), func(on bool) {
		if config.Current.ShowInDock == on {
			return
		}
		config.Current.ShowInDock = on
		applyDockVisibility()
		save()
		w.RequestFocus()
	})
	dockCheck.SetChecked(config.Current.ShowInDock)
	if runtime.GOOS != "darwin" {
		dockCheck.Hide() // Dock yalnızca macOS'ta var
	}

	// Oturum açılışında başlatma: durum ayar dosyasında değil, macOS'ta tutulur.
	loginHint := hintText("")
	loginHint.Hide()
	var loginCheck *widget.Check
	var refreshLoginState func()
	refreshLoginState = func() {
		st := loginItemState()
		loginCheck.OnChanged = nil
		loginCheck.SetChecked(st == loginEnabled || st == loginRequiresApprove)
		loginCheck.OnChanged = func(on bool) {
			if !setLaunchAtLogin(on) {
				dialog.ShowInformation(T("Oturum açılışında başlat"), T("Ayar değiştirilemedi. Uygulamayı Uygulamalar klasörüne taşıyıp tekrar deneyin."), w)
			}
			refreshLoginState()
		}
		switch st {
		case loginRequiresApprove:
			loginHint.SetText(T("macOS onay bekliyor: Sistem Ayarları → Genel → Giriş Öğeleri'nden AI Tempo'ya izin verin."))
			loginHint.Show()
		case loginNotFound, loginUnsupported:
			loginCheck.Disable()
			if runtime.GOOS == "darwin" {
				loginHint.SetText(T("Bu seçenek yalnızca Uygulamalar klasöründeki AI Tempo.app için kullanılabilir (macOS 13+)."))
			} else {
				loginHint.SetText(T("Bu sistemde kullanılamıyor."))
			}
			loginHint.Show()
		default:
			loginHint.Hide()
		}
	}
	loginCheck = widget.NewCheck(T("Oturum açılışında başlat"), nil)
	refreshLoginState()

	historyOptions := []int{2, 7, 14, 35, 60, 90}
	historyLabels := make([]string, len(historyOptions))
	for i, d := range historyOptions {
		historyLabels[i] = Tf("%d gün", d)
	}
	historySelect := widget.NewSelect(historyLabels, func(sel string) {
		for i, l := range historyLabels {
			if l == sel && config.Current.HistoryDays != historyOptions[i] {
				config.Current.HistoryDays = historyOptions[i]
				history.SetRetention(historyOptions[i])
				save()
			}
		}
	})
	historySelect.SetSelected(Tf("%d gün", config.Current.HistoryDays))

	general := newRoundedCard(container.NewVBox(
		sectionTitle(T("Genel")),
		widget.NewForm(
			widget.NewFormItem(T("Dil"), langSelect),
			widget.NewFormItem(T("Otomatik yenileme"), refreshSelect),
			widget.NewFormItem(T("Geçmişi sakla"), historySelect),
			widget.NewFormItem("", dockCheck),
			widget.NewFormItem("", container.NewVBox(loginCheck, loginHint)),
		),
		hintText(T("Her hesap bu aralıkla, sırayla sorgulanır. Üst üste 5 kez hata alan hesap otomatik sorgulanmaz; sayacı hesabın sekmesinden sıfırlayabilirsiniz. Sorgu geçmişi ve grafik verisi seçilen süre kadar saklanır; aylık dönemler için en az 35 gün önerilir.")),
	))

	accountsHeader := container.NewBorder(nil, nil, sectionTitle(T("Hesaplar")), addBtn)
	aboutBtn := widget.NewButtonWithIcon(T("Hakkında"), theme.InfoIcon(), func() { showAbout(w) })
	aboutBtn.Importance = widget.LowImportance
	footer := container.NewBorder(nil, nil, nil, aboutBtn,
		hintText("🔒 "+secretStoreNote()))

	content := container.NewVBox(
		general,
		container.NewPadded(accountsHeader),
		accountList,
		container.NewPadded(footer),
	)
	w.SetContent(container.NewScroll(container.NewPadded(content)))
	w.Show()
	rememberWindowFrame(w, "AITempoSettingsWindow.v2")
	activateApp()
}

// showAccountEditor, hesap ekleme/düzenleme diyaloğunu açar. existing nil ise yeni
// hesap eklenir. onSave'e anahtar alanı boş bırakıldıysa newKey="" gelir (düzenlemede
// mevcut anahtar korunur).
func showAccountEditor(parent fyne.Window, existing *config.Account, onSave func(acc config.Account, newKey string)) {
	acc := config.Account{Provider: "claude", Enabled: true}
	if existing != nil {
		acc = *existing
	}

	providerIDs := []string{"claude", "cursor", "chatgpt"}
	providerNames := make([]string, len(providerIDs))
	for i, id := range providerIDs {
		providerNames[i] = provider.Get(id).DisplayName()
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder(T("örn. e-posta adresiniz"))
	nameEntry.SetText(acc.Name)

	// Çok satırlı ve görünür: uzun token'ın başı/sonu kontrol edilebilsin.
	keyEntry := widget.NewMultiLineEntry()
	keyEntry.Wrapping = fyne.TextWrapBreak
	keyEntry.SetMinRowsVisible(4)
	keyInfo := canvas.NewText("", colorMuted)
	keyInfo.TextSize = 11
	keyEntry.OnChanged = func(string) {
		keyInfo.Text, keyInfo.Color = keySummary(normalizeKey(keyEntry.Text))
		keyInfo.Refresh()
	}
	if existing != nil && existing.SessionKey != "" {
		keyEntry.SetPlaceHolder(T("Değiştirmek için yeni anahtar girin"))
	} else {
		keyEntry.SetPlaceHolder(T("Oturum anahtarı / token"))
	}

	orgEntry := widget.NewEntry()
	orgEntry.SetText(acc.OrganizationID)

	keyHelp := hintText("")
	orgHelp := hintText("")
	keyHelp.Wrapping = fyne.TextWrapWord
	orgHelp.Wrapping = fyne.TextWrapWord

	// Gelişmiş: sağlayıcıya özel ek seçenekler (şimdilik yalnızca ChatGPT).
	resetCreditsCheck := widget.NewCheck(T("Codex sıfırlama haklarını göster"), nil)
	resetCreditsCheck.SetChecked(acc.ShowResetCredits)
	advanced := widget.NewAccordion(widget.NewAccordionItem(T("Gelişmiş"), container.NewVBox(
		resetCreditsCheck,
		hintText(T("Kullanılabilir ücretsiz limit sıfırlama hakkı sayısını ve son kullanma tarihini gösterir. Her yenilemede ek bir istek yapar.")),
	)))

	snippetBtn := widget.NewButtonWithIcon(T("Konsoldan al"), theme.ContentCopyIcon(), func() {
		showConsoleSnippet(parent, acc.Provider)
	})
	snippetBtn.Importance = widget.LowImportance

	providerSelect := widget.NewSelect(providerNames, nil)
	providerSelect.OnChanged = func(sel string) {
		for i, n := range providerNames {
			if n == sel {
				acc.Provider = providerIDs[i]
			}
		}
		h := providerHelp[acc.Provider]
		keyHelp.SetText(T(h.key))
		orgHelp.SetText(T(h.org))
		if acc.Provider == "cursor" {
			orgEntry.Disable()
		} else {
			orgEntry.Enable()
		}
		if acc.Provider == "chatgpt" {
			advanced.Show()
		} else {
			advanced.Hide()
		}
	}
	providerSelect.SetSelected(provider.Get(acc.Provider).DisplayName())

	enabledCheck := widget.NewCheck(T("Menüde ve pencerede göster"), nil)
	enabledCheck.SetChecked(acc.Enabled)

	form := widget.NewForm(
		widget.NewFormItem(T("Sağlayıcı"), providerSelect),
		widget.NewFormItem(T("İsim"), nameEntry),
		widget.NewFormItem(T("Oturum anahtarı"), container.NewVBox(keyEntry, keyInfo, keyHelp, container.NewHBox(snippetBtn))),
		widget.NewFormItem("Organization ID", container.NewVBox(orgEntry, orgHelp)),
		widget.NewFormItem("", enabledCheck),
	)
	// Test Et: formdaki (henüz kaydedilmemiş) bilgilerle tek bir sorgu yapar.
	testResult := widget.NewLabel("")
	testResult.Wrapping = fyne.TextWrapWord
	var testBtn *widget.Button
	testBtn = widget.NewButtonWithIcon(T("Test Et"), theme.MediaPlayIcon(), func() {
		probe := acc
		probe.OrganizationID = strings.TrimSpace(orgEntry.Text)
		probe.ShowResetCredits = false
		probe.SessionKey = normalizeKey(keyEntry.Text)
		if probe.Provider == "cursor" {
			probe.OrganizationID = ""
			probe.SessionKey = strings.ReplaceAll(probe.SessionKey, "%3A%3A", "::")
		}
		if probe.SessionKey == "" && existing != nil {
			probe.SessionKey = existing.SessionKey // alan boşsa kayıtlı anahtarla dene
		}
		if probe.SessionKey == "" {
			testResult.SetText("✗ " + T("Oturum anahtarı gerekli."))
			return
		}
		testBtn.Disable()
		testResult.SetText(T("Test ediliyor…"))
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			info := provider.Get(probe.Provider).Query(ctx, probe)
			cancel()
			fyne.Do(func() {
				testBtn.Enable()
				if info.Success {
					var parts []string
					for _, m := range info.Metrics {
						parts = append(parts, fmt.Sprintf("%s %.0f%%", m.Label, m.Percent))
					}
					testResult.SetText("✓ " + T("Çalışıyor:") + " " + strings.Join(parts, " · "))
				} else {
					testResult.SetText("✗ " + info.Error)
				}
			})
		}()
	})
	editor := container.NewVBox(form, advanced, container.NewBorder(nil, nil, testBtn, nil, testResult))

	title := T("Hesap Ekle")
	if existing != nil {
		title = T("Hesabı Düzenle")
	}
	d := dialog.NewCustomConfirm(title, T("Kaydet"), T("Vazgeç"), container.NewPadded(editor), func(ok bool) {
		if !ok {
			return
		}
		acc.Name = strings.TrimSpace(nameEntry.Text)
		acc.OrganizationID = strings.TrimSpace(orgEntry.Text)
		acc.Enabled = enabledCheck.Checked
		acc.ShowResetCredits = acc.Provider == "chatgpt" && resetCreditsCheck.Checked
		key := normalizeKey(keyEntry.Text)
		if acc.Provider == "cursor" {
			acc.OrganizationID = ""
			key = strings.ReplaceAll(key, "%3A%3A", "::")
		}

		var problems []string
		if acc.Name == "" {
			problems = append(problems, T("İsim boş olamaz."))
		}
		if key == "" && (existing == nil || existing.SessionKey == "") {
			problems = append(problems, T("Oturum anahtarı gerekli."))
		}
		if acc.Provider == "claude" && acc.OrganizationID == "" {
			problems = append(problems, T("Claude için Organization ID gerekli."))
		}
		if len(problems) > 0 {
			dialog.ShowInformation(T("Eksik bilgi"), strings.Join(problems, "\n"), parent)
			return
		}
		onSave(acc, key)
	}, parent)
	d.Resize(fyne.NewSize(640, 560))
	d.Show()
}

// refreshLabel, yenileme aralığını okunur hale getirir (60 ve katları saat olarak).
func refreshLabel(minutes int) string {
	if minutes >= 60 && minutes%60 == 0 {
		return Tf("%d saat", minutes/60)
	}
	return Tf("%d dakika", minutes)
}

func sectionTitle(text string) fyne.CanvasObject {
	t := canvas.NewText(text, nil)
	t.TextSize = 15
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func hintText(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// secretStoreNote, anahtarların işletim sisteminde nerede saklandığını anlatır.
func secretStoreNote() string {
	if runtime.GOOS == "windows" {
		return T("Oturum anahtarları diske yazılmaz, Windows Kimlik Bilgisi Yöneticisi'nde saklanır.")
	}
	return T("Oturum anahtarları diske yazılmaz, macOS Anahtar Zinciri'nde (Keychain) saklanır.")
}

// normalizeKey, yapıştırılan anahtardan boşlukları, satır sonlarını ve başındaki
// "Bearer " önekini temizler.
func normalizeKey(s string) string {
	s = strings.Join(strings.Fields(s), "")
	s = strings.TrimPrefix(s, "Bearer")
	return s
}

// keySummary, anahtarın uzunluğunu, başını ve sonunu gösterir; kopyalarken
// kesilmiş ("…" içeren) anahtarlar için uyarı döndürür.
func keySummary(key string) (string, color.Color) {
	if key == "" {
		return "", colorMuted
	}
	r := []rune(key)
	preview := key
	if len(r) > 24 {
		preview = string(r[:10]) + " … " + string(r[len(r)-10:])
	}
	text := Tf("%d karakter · %s", len(r), preview)
	if strings.ContainsRune(key, '…') {
		return "⚠ " + T("Anahtarda “…” var: kopyalarken kesilmiş olabilir.") + "  " + text, colorDanger
	}
	return "✓ " + text, colorGood
}

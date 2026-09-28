package gui

import (
	"fmt"
	"strings"

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

var settingsWindow fyne.Window

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
	w.SetOnClosed(func() { settingsWindow = nil })
	w.Resize(fyne.NewSize(560, 560))

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
					if newKey != "" {
						if err := config.KeychainSet(updated.ID, newKey); err != nil {
							dialog.ShowError(fmt.Errorf("%s: %w", T("Anahtar Keychain'e kaydedilemedi"), err), w)
							return
						}
						updated.SessionKey = newKey
					}
					config.Current.Accounts[idx] = updated
					store.Forget(updated.ID)
					save()
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

	langNames := make([]string, len(i18n.Languages))
	for i, l := range i18n.Languages {
		langNames[i] = l.Name
	}
	langSelect := widget.NewSelect(langNames, nil)
	for _, l := range i18n.Languages {
		if l.Code == i18n.Lang() {
			langSelect.SetSelected(l.Name)
		}
	}
	langSelect.OnChanged = func(sel string) {
		for _, l := range i18n.Languages {
			if l.Name == sel && l.Code != i18n.Lang() {
				i18n.Set(l.Code)
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
		),
		hintText(T("Her hesap bu aralıkla, sırayla sorgulanır. Üst üste 5 kez hata alan hesap otomatik sorgulanmaz; sayacı hesabın sekmesinden sıfırlayabilirsiniz. Sorgu geçmişi ve grafik verisi seçilen süre kadar saklanır; aylık dönemler için en az 35 gün önerilir.")),
	))

	accountsHeader := container.NewBorder(nil, nil, sectionTitle(T("Hesaplar")), addBtn)
	footer := hintText("🔒 " + T("Oturum anahtarları diske yazılmaz, macOS Anahtar Zinciri'nde (Keychain) saklanır."))

	content := container.NewVBox(
		general,
		container.NewPadded(accountsHeader),
		accountList,
		container.NewPadded(footer),
	)
	w.SetContent(container.NewScroll(container.NewPadded(content)))
	w.Show()
	rememberWindowFrame(w, "AITempoSettingsWindow")
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

	keyEntry := widget.NewPasswordEntry()
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
		widget.NewFormItem(T("Oturum anahtarı"), container.NewVBox(keyEntry, keyHelp, container.NewHBox(snippetBtn))),
		widget.NewFormItem("Organization ID", container.NewVBox(orgEntry, orgHelp)),
		widget.NewFormItem("", enabledCheck),
	)
	editor := container.NewVBox(form, advanced)

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
		key := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(keyEntry.Text), "Bearer "))
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
	d.Resize(fyne.NewSize(520, 420))
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

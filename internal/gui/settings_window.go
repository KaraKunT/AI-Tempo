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
	"ai-tempo/internal/provider"
)

var refreshOptions = []int{5, 10, 15, 30, 60}

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
	w := app.NewWindow("Ayarlar")
	settingsWindow = w
	w.SetOnClosed(func() { settingsWindow = nil })
	w.Resize(fyne.NewSize(560, 560))

	accountList := container.NewVBox()
	var rebuild func()

	save := func() {
		if err := config.Current.Save(); err != nil {
			dialog.ShowError(fmt.Errorf("Ayarlar kaydedilemedi: %w", err), w)
			return
		}
		onChanged()
		rebuild()
	}

	rebuild = func() {
		accountList.RemoveAll()
		if len(config.Current.Accounts) == 0 {
			empty := canvas.NewText("Henüz hesap yok.", colorMuted)
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
				detail += " · ⚠ anahtar yok"
			}
			sub := canvas.NewText(detail, colorMuted)
			sub.TextSize = 11

			enabled := widget.NewCheck("Etkin", func(on bool) {
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
							dialog.ShowError(fmt.Errorf("Anahtar Keychain'e kaydedilemedi: %w", err), w)
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
				dialog.ShowConfirm("Hesabı sil", fmt.Sprintf("“%s” silinsin mi? Anahtarı da Keychain'den kaldırılır.", acc.Name), func(ok bool) {
					if !ok {
						return
					}
					config.KeychainDelete(acc.ID)
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

	addBtn := widget.NewButtonWithIcon("Hesap Ekle", theme.ContentAddIcon(), func() {
		showAccountEditor(w, nil, func(acc config.Account, newKey string) {
			acc.ID = config.NewAccountID()
			if err := config.KeychainSet(acc.ID, newKey); err != nil {
				dialog.ShowError(fmt.Errorf("Anahtar Keychain'e kaydedilemedi: %w", err), w)
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
		labels[i] = fmt.Sprintf("%d dakika", m)
	}
	refreshSelect := widget.NewSelect(labels, func(sel string) {
		for i, l := range labels {
			if l == sel && config.Current.RefreshMinutes != refreshOptions[i] {
				config.Current.RefreshMinutes = refreshOptions[i]
				save()
			}
		}
	})
	refreshSelect.SetSelected(fmt.Sprintf("%d dakika", config.Current.RefreshMinutes))

	general := newRoundedCard(container.NewVBox(
		sectionTitle("Genel"),
		widget.NewForm(widget.NewFormItem("Otomatik yenileme", refreshSelect)),
		hintText("Her hesap bu aralıkla, sırayla sorgulanır. Hata alan hesaplarda aralık otomatik uzar."),
	))

	accountsHeader := container.NewBorder(nil, nil, sectionTitle("Hesaplar"), addBtn)
	footer := hintText("🔒 Oturum anahtarları diske yazılmaz, macOS Anahtar Zinciri'nde (Keychain) saklanır.")

	content := container.NewVBox(
		general,
		container.NewPadded(accountsHeader),
		accountList,
		container.NewPadded(footer),
	)
	w.SetContent(container.NewScroll(container.NewPadded(content)))
	w.Show()
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
	nameEntry.SetPlaceHolder("örn. e-posta adresiniz")
	nameEntry.SetText(acc.Name)

	keyEntry := widget.NewPasswordEntry()
	if existing != nil && existing.SessionKey != "" {
		keyEntry.SetPlaceHolder("Değiştirmek için yeni anahtar girin")
	} else {
		keyEntry.SetPlaceHolder("Oturum anahtarı / token")
	}

	orgEntry := widget.NewEntry()
	orgEntry.SetText(acc.OrganizationID)

	keyHelp := hintText("")
	orgHelp := hintText("")
	keyHelp.Wrapping = fyne.TextWrapWord
	orgHelp.Wrapping = fyne.TextWrapWord

	providerSelect := widget.NewSelect(providerNames, nil)
	providerSelect.OnChanged = func(sel string) {
		for i, n := range providerNames {
			if n == sel {
				acc.Provider = providerIDs[i]
			}
		}
		h := providerHelp[acc.Provider]
		keyHelp.SetText(h.key)
		orgHelp.SetText(h.org)
		if acc.Provider == "cursor" {
			orgEntry.Disable()
		} else {
			orgEntry.Enable()
		}
	}
	providerSelect.SetSelected(provider.Get(acc.Provider).DisplayName())

	enabledCheck := widget.NewCheck("Menüde ve pencerede göster", nil)
	enabledCheck.SetChecked(acc.Enabled)

	form := widget.NewForm(
		widget.NewFormItem("Sağlayıcı", providerSelect),
		widget.NewFormItem("İsim", nameEntry),
		widget.NewFormItem("Oturum anahtarı", container.NewVBox(keyEntry, keyHelp)),
		widget.NewFormItem("Organization ID", container.NewVBox(orgEntry, orgHelp)),
		widget.NewFormItem("", enabledCheck),
	)

	title := "Hesap Ekle"
	if existing != nil {
		title = "Hesabı Düzenle"
	}
	d := dialog.NewCustomConfirm(title, "Kaydet", "Vazgeç", container.NewPadded(form), func(ok bool) {
		if !ok {
			return
		}
		acc.Name = strings.TrimSpace(nameEntry.Text)
		acc.OrganizationID = strings.TrimSpace(orgEntry.Text)
		acc.Enabled = enabledCheck.Checked
		key := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(keyEntry.Text), "Bearer "))
		if acc.Provider == "cursor" {
			acc.OrganizationID = ""
			key = strings.ReplaceAll(key, "%3A%3A", "::")
		}

		var problems []string
		if acc.Name == "" {
			problems = append(problems, "İsim boş olamaz.")
		}
		if key == "" && (existing == nil || existing.SessionKey == "") {
			problems = append(problems, "Oturum anahtarı gerekli.")
		}
		if acc.Provider == "claude" && acc.OrganizationID == "" {
			problems = append(problems, "Claude için Organization ID gerekli.")
		}
		if len(problems) > 0 {
			dialog.ShowInformation("Eksik bilgi", strings.Join(problems, "\n"), parent)
			return
		}
		onSave(acc, key)
	}, parent)
	d.Resize(fyne.NewSize(520, 420))
	d.Show()
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

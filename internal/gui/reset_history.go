package gui

import (
	"context"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"ai-tempo/internal/config"
	"ai-tempo/internal/provider"
)

// historyTTL, çekilmiş geçmişin yeniden sorgulanmadan gösterileceği süredir.
const historyTTL = 10 * time.Minute

// resetCardState, sıfırlama kartının hesap başına durumudur. Kart her otomatik
// yenilemede yeniden çizildiği için seçili sekme ve geçmiş burada saklanır;
// böylece geçmiş yalnızca "Geçmiş" sekmesi açılınca (ve TTL dolunca) çekilir.
type resetCardState struct {
	showHistory bool
	history     *provider.ResetHistory
	fetchedAt   time.Time
	loading     bool
	err         string
}

var (
	resetStatesMu sync.Mutex
	resetStates   = map[string]*resetCardState{}
)

func resetStateFor(id string) *resetCardState {
	resetStatesMu.Lock()
	defer resetStatesMu.Unlock()
	st, ok := resetStates[id]
	if !ok {
		st = &resetCardState{}
		resetStates[id] = st
	}
	return st
}

// newResetCreditsCard, ChatGPT arayüzündeki "Kullanım limiti yenileme hakları"
// bölümünün benzeri: Kullanılabilir / Geçmiş sekmeli bir kart.
func newResetCreditsCard(account config.Account, rc *provider.ResetCredits) fyne.CanvasObject {
	st := resetStateFor(account.ID)

	title := canvas.NewText(T("Kullanım limiti yenileme hakları"), nil)
	title.TextSize = 16
	title.TextStyle = fyne.TextStyle{Bold: true}
	desc := widget.NewLabel(T("Bir sıfırlama kullanarak 5 saatlik veya haftalık limitinizi ya da her ikisini yenileyin."))
	desc.Wrapping = fyne.TextWrapWord

	body := container.NewVBox()
	periodLabel := canvas.NewText(T("Son 30 gün"), colorMuted)
	periodLabel.TextSize = 12

	availableBtn := widget.NewButton(Tf("Kullanılabilir  %d", len(rc.Available)), nil)
	historyBtn := widget.NewButton(T("Geçmiş"), nil)

	var refresh func()
	refresh = func() {
		if st.showHistory {
			availableBtn.Importance, historyBtn.Importance = widget.LowImportance, widget.MediumImportance
			periodLabel.Show()
		} else {
			availableBtn.Importance, historyBtn.Importance = widget.MediumImportance, widget.LowImportance
			periodLabel.Hide()
		}
		availableBtn.Refresh()
		historyBtn.Refresh()

		body.RemoveAll()
		switch {
		case !st.showHistory:
			addAvailableRows(body, rc)
		case st.loading:
			body.Add(container.NewPadded(widget.NewProgressBarInfinite()))
		case st.err != "":
			body.Add(mutedRow(st.err))
		default:
			addHistoryRows(body, st.history)
		}
		body.Refresh()
	}

	availableBtn.OnTapped = func() { st.showHistory = false; refresh() }
	historyBtn.OnTapped = func() {
		st.showHistory = true
		if !st.loading && (st.history == nil || time.Since(st.fetchedAt) > historyTTL) {
			st.loading, st.err = true, ""
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				h, err := provider.FetchChatGPTResetHistory(ctx, account)
				fyne.Do(func() {
					st.loading = false
					if err != nil {
						st.err = T("Geçmiş alınamadı:") + " " + err.Error()
					} else {
						st.history, st.fetchedAt = &h, time.Now()
					}
					refresh()
				})
			}()
		}
		refresh()
	}
	refresh()

	tabs := container.NewBorder(nil, nil, container.NewHBox(availableBtn, historyBtn), container.NewCenter(periodLabel))
	return newRoundedCard(container.NewVBox(title, desc, widget.NewSeparator(), tabs, body))
}

func addAvailableRows(body *fyne.Container, rc *provider.ResetCredits) {
	if rc.Error != "" {
		body.Add(mutedRow(rc.Error))
		return
	}
	if len(rc.Available) == 0 {
		body.Add(mutedRow(T("Kullanılabilir sıfırlama hakkı yok.")))
		return
	}
	for _, c := range rc.Available {
		body.Add(widget.NewSeparator())
		name := canvas.NewText(c.Title, nil)
		name.TextStyle = fyne.TextStyle{Bold: true}
		expiry := canvas.NewText(T("Bitiş zamanı")+" "+provider.ShortDateTime(c.ExpiresAt), colorMuted)
		expiry.TextSize = 12
		body.Add(container.NewPadded(container.NewVBox(name, expiry)))
	}
}

func addHistoryRows(body *fyne.Container, h *provider.ResetHistory) {
	if h == nil || len(h.Events) == 0 {
		body.Add(mutedRow(T("Bu dönemde kayıt yok.")))
		return
	}
	for _, e := range h.Events {
		label := T("Sıfırlama hakkı alındı")
		if e.Kind == "used" {
			label = T("Sıfırlama hakkı kullanıldı")
		}
		name := canvas.NewText(label, nil)
		name.TextStyle = fyne.TextStyle{Bold: true}

		day := canvas.NewText(provider.ShortDate(e.At), nil)
		day.TextStyle = fyne.TextStyle{Bold: true}
		day.Alignment = fyne.TextAlignTrailing
		clock := canvas.NewText(provider.ClockWithZone(e.At), colorMuted)
		clock.TextSize = 12
		clock.Alignment = fyne.TextAlignTrailing

		body.Add(widget.NewSeparator())
		body.Add(container.NewPadded(container.NewBorder(nil, nil, container.NewCenter(name), container.NewVBox(day, clock))))
	}
}

func mutedRow(text string) fyne.CanvasObject {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

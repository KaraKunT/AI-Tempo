package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"ai-tempo/internal/history"
)

// maxLogRows, açılır geçmiş listesinde gösterilen en fazla kayıt sayısıdır.
const maxLogRows = 300

// newLogSection, sekmenin altındaki açılır "Sorgu Geçmişi" bölümünü oluşturur.
// Dönen refresh fonksiyonu kayıtları veritabanından yeniden okur; bölüm açık
// değilse yalnızca başlıktaki sayıyı günceller.
func newLogSection(accountID string) (fyne.CanvasObject, func()) {
	rows := container.NewVBox()
	open := false
	selected := rangeToday
	var header *widget.Button
	var refresh func()

	filter := newRangeSelector([]timeRange{rangeToday, rangeYesterday, rangeWeek, rangeMonth}, selected, func(r timeRange) {
		selected = r
		refresh()
	})
	panel := container.NewVBox(container.NewCenter(filter), rows)
	panel.Hide()

	refresh = func() {
		from, to := selected.bounds(time.Now())
		entries := history.List(accountID, from, to)
		failed := 0
		for _, e := range entries {
			if !e.Success {
				failed++
			}
		}
		arrow := "▸"
		if open {
			arrow = "▾"
		}
		title := fmt.Sprintf("%s  %s · %s · %s", arrow, T("Sorgu Geçmişi"), T(string(selected)), Tf("%d kayıt", len(entries)))
		if failed > 0 {
			title += " " + Tf("(%d hata)", failed)
		}
		header.SetText(title)
		if open {
			rows.Objects = logRows(entries)
			rows.Refresh()
		}
	}
	header = widget.NewButton("", func() {
		open = !open
		if open {
			panel.Show()
		} else {
			panel.Hide()
		}
		refresh()
	})
	header.Alignment = widget.ButtonAlignLeading
	header.Importance = widget.LowImportance
	refresh()

	return newRoundedCard(container.NewVBox(header, panel)), refresh
}

// logRows, kayıtları gün başlıklarıyla ayrılmış satırlara dönüştürür.
func logRows(entries []history.Entry) []fyne.CanvasObject {
	if len(entries) == 0 {
		t := canvas.NewText(T("Bu aralıkta kayıt yok."), colorMuted)
		t.TextSize = 12
		return []fyne.CanvasObject{container.NewPadded(t)}
	}
	var out []fyne.CanvasObject
	lastDay := ""
	for i, e := range entries {
		if i == maxLogRows {
			more := canvas.NewText(Tf("… %d kayıt daha", len(entries)-maxLogRows), colorMuted)
			more.TextSize = 11
			out = append(out, container.NewCenter(more))
			break
		}
		if day := dayLabel(e.At); day != lastDay {
			lastDay = day
			h := canvas.NewText(day, colorMuted)
			h.TextSize = 11
			h.TextStyle = fyne.TextStyle{Bold: true}
			out = append(out, container.New(layout.NewCustomPaddedLayout(8, 2, 4, 0), h))
		}
		out = append(out, logRow(e))
	}
	return out
}

// logRow, tek bir kaydı "● saat [tetik] mesaj ... süre" düzeninde çizer.
func logRow(e history.Entry) fyne.CanvasObject {
	col := colorGood
	if !e.Success {
		col = colorDanger
	}
	dot := canvas.NewCircle(col)
	dotBox := container.NewGridWrap(fyne.NewSquareSize(8), dot)

	timeText := canvas.NewText(e.At.Format("15:04:05"), nil)
	timeText.TextSize = 12
	timeText.TextStyle = fyne.TextStyle{Monospace: true}

	trigger := canvas.NewText(T(e.Trigger), colorAccent)
	trigger.TextSize = 10
	trigger.TextStyle = fyne.TextStyle{Bold: true}
	pill := canvas.NewRectangle(withAlpha(colorAccent, 0x26))
	pill.CornerRadius = 6
	chip := container.NewStack(pill, container.New(layout.NewCustomPaddedLayout(1, 1, 6, 6), trigger))

	msg := canvas.NewText(e.Message, nil)
	if !e.Success {
		msg.Color = colorDanger
	}
	msg.TextSize = 12

	dur := canvas.NewText(formatDuration(e.Duration), colorMuted)
	dur.TextSize = 11
	dur.TextStyle = fyne.TextStyle{Monospace: true}

	left := container.NewHBox(container.NewCenter(dotBox), timeText, container.NewCenter(chip))
	row := container.NewBorder(nil, nil, left, dur, msg)
	return container.New(layout.NewCustomPaddedLayout(2, 2, 4, 4), row)
}

// dayLabel, kaydın gününü "Bugün", "Dün" veya tarih olarak döndürür.
func dayLabel(t time.Time) string {
	now := time.Now()
	y, m, d := t.Date()
	ny, nm, nd := now.Date()
	switch {
	case y == ny && m == nm && d == nd:
		return T("BUGÜN")
	case now.AddDate(0, 0, -1).Day() == d && now.AddDate(0, 0, -1).Month() == m:
		return T("DÜN")
	}
	return t.Format("02.01.2006")
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return Tf("%.1f sn", d.Seconds())
}

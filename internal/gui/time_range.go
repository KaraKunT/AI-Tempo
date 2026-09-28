package gui

import (
	"time"

	"fyne.io/fyne/v2/widget"
)

// timeRange, geçmiş ve grafik filtrelerinde seçilebilen zaman aralığıdır.
type timeRange string

const (
	rangePeriod    timeRange = "Dönem" // yalnızca grafik: göstergenin kendi dönemi (başı → reset)
	rangeToday     timeRange = "Bugün"
	rangeYesterday timeRange = "Dün"
	rangeWeek      timeRange = "Hafta"
	rangeMonth     timeRange = "Ay"
)

// bounds, aralığın [from, to) sınırlarını döndürür. "Hafta" ve "Ay" son 7 ve
// son 30 günü kapsar. rangePeriod için sıfır değerler döner (çağıran belirler).
func (r timeRange) bounds(now time.Time) (from, to time.Time) {
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	switch r {
	case rangeToday:
		return today, now.Add(time.Minute)
	case rangeYesterday:
		return today.AddDate(0, 0, -1), today
	case rangeWeek:
		return now.AddDate(0, 0, -7), now.Add(time.Minute)
	case rangeMonth:
		return now.AddDate(0, 0, -30), now.Add(time.Minute)
	}
	return time.Time{}, time.Time{}
}

// newRangeSelector, yatay seçenek düğmeleri oluşturur; seçim değişince onChange çağrılır.
func newRangeSelector(options []timeRange, selected timeRange, onChange func(timeRange)) *widget.RadioGroup {
	labels := make([]string, len(options))
	byLabel := map[string]timeRange{}
	for i, o := range options {
		labels[i] = T(string(o))
		byLabel[labels[i]] = o
	}
	rg := widget.NewRadioGroup(labels, nil)
	rg.Horizontal = true
	rg.Required = true
	rg.SetSelected(T(string(selected)))
	rg.OnChanged = func(s string) { onChange(byLabel[s]) }
	return rg
}

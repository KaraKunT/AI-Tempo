package gui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"ai-tempo/internal/history"
	"ai-tempo/internal/provider"
)

const (
	chartHeight   = 170
	chartLeftPad  = 34 // y ekseni etiketleri
	chartBottomPd = 18 // x ekseni etiketleri
	chartTopPad   = 6
)

// usageChart, bir kota göstergesinin dönem başından reset zamanına kadar
// kullanımını çizer: gri kılavuz çizgiler, mor kesikli "ideal tempo" çizgisi
// (0'dan 100'e doğrusal), renkli kullanım çizgisi ve "şimdi" işareti.
type usageChart struct {
	widget.BaseWidget
	start, end time.Time
	samples    []history.Sample
	showPace   bool // ideal tempo çizgisi yalnızca göstergenin kendi döneminde anlamlı
}

func newUsageChart(start, end time.Time, samples []history.Sample, showPace bool) *usageChart {
	c := &usageChart{start: start, end: end, samples: samples, showPace: showPace}
	c.ExtendBaseWidget(c)
	return c
}

func (c *usageChart) CreateRenderer() fyne.WidgetRenderer {
	return &usageChartRenderer{chart: c}
}

type usageChartRenderer struct {
	chart   *usageChart
	objects []fyne.CanvasObject
}

func (r *usageChartRenderer) MinSize() fyne.Size { return fyne.NewSize(200, chartHeight) }
func (r *usageChartRenderer) Refresh()           { r.Layout(r.chart.Size()); canvas.Refresh(r.chart) }
func (r *usageChartRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}
func (r *usageChartRenderer) Destroy() {}

// Layout, tüm çizim nesnelerini boyuta göre yeniden oluşturur.
func (r *usageChartRenderer) Layout(size fyne.Size) {
	c := r.chart
	var objs []fyne.CanvasObject
	plotW := size.Width - chartLeftPad
	plotH := size.Height - chartBottomPd - chartTopPad
	if plotW <= 0 || plotH <= 0 || !c.end.After(c.start) {
		r.objects = nil
		return
	}
	span := c.end.Sub(c.start).Seconds()
	xOf := func(t time.Time) float32 {
		f := t.Sub(c.start).Seconds() / span
		f = max(0, min(1, f))
		return chartLeftPad + float32(f)*plotW
	}
	yOf := func(p float64) float32 {
		p = max(0, min(100, p))
		return chartTopPad + plotH - float32(p/100)*plotH
	}
	line := func(x1, y1, x2, y2 float32, col color.Color, w float32) {
		l := canvas.NewLine(col)
		l.StrokeWidth = w
		l.Position1 = fyne.NewPos(x1, y1)
		l.Position2 = fyne.NewPos(x2, y2)
		objs = append(objs, l)
	}
	label := func(text string, x, y float32, align fyne.TextAlign) {
		t := canvas.NewText(text, colorMuted)
		t.TextSize = 10
		t.Alignment = align
		ts := t.MinSize()
		switch align {
		case fyne.TextAlignTrailing:
			x -= ts.Width
		case fyne.TextAlignCenter:
			x -= ts.Width / 2
		}
		t.Move(fyne.NewPos(x, y-ts.Height/2))
		t.Resize(ts)
		objs = append(objs, t)
	}

	// Kılavuz çizgileri ve y etiketleri
	grid := withAlpha(colorMuted, 0x40)
	for _, p := range []float64{0, 25, 50, 75, 100} {
		y := yOf(p)
		line(chartLeftPad, y, size.Width, y, grid, 1)
		label(fmt.Sprintf("%%%.0f", p), chartLeftPad-4, y, fyne.TextAlignTrailing)
	}

	// İdeal tempo: dönem boyunca eşit kullanım (kesikli)
	pace := withAlpha(colorAccent, 0x90)
	const dashes = 40
	for i := 0; c.showPace && i < dashes; i += 2 {
		f1, f2 := float32(i)/dashes, float32(i+1)/dashes
		line(chartLeftPad+f1*plotW, chartTopPad+plotH-f1*plotH,
			chartLeftPad+f2*plotW, chartTopPad+plotH-f2*plotH, pace, 1.5)
	}

	// Şimdi işareti
	now := time.Now()
	if now.After(c.start) && now.Before(c.end) {
		x := xOf(now)
		line(x, chartTopPad, x, chartTopPad+plotH, withAlpha(colorMuted, 0x80), 1)
		label(T("şimdi"), x, size.Height-chartBottomPd/2, fyne.TextAlignCenter)
	}

	// Kullanım çizgisi
	var prev *history.Sample
	for i := range c.samples {
		s := &c.samples[i]
		if prev != nil {
			line(xOf(prev.At), yOf(prev.Percent), xOf(s.At), yOf(s.Percent), severityColor(s.Percent), 2.5)
		}
		prev = s
	}
	if n := len(c.samples); n > 0 {
		last := c.samples[n-1]
		dot := canvas.NewCircle(severityColor(last.Percent))
		dot.Resize(fyne.NewSquareSize(7))
		dot.Move(fyne.NewPos(xOf(last.At)-3.5, yOf(last.Percent)-3.5))
		objs = append(objs, dot)
	}

	// x ekseni: dönem başı ve reset zamanı
	axisLabel := provider.ShortDateTime
	if c.end.Sub(c.start) <= 25*time.Hour {
		axisLabel = func(t time.Time) string { return t.In(time.Local).Format("15:04") }
	}
	label(axisLabel(c.start), chartLeftPad, size.Height-chartBottomPd/2, fyne.TextAlignLeading)
	label(axisLabel(c.end), size.Width, size.Height-chartBottomPd/2, fyne.TextAlignTrailing)

	r.objects = objs
}

// newChartsSection, hesabın her göstergesi için seçili aralıkta bir grafik kartı
// döndürür. "Dönem" seçiliyse her gösterge kendi dönemini (başı → reset) çizer.
func newChartsSection(accountID string, info *provider.RateLimitInfo, r timeRange) []fyne.CanvasObject {
	if info == nil || !info.Success {
		return nil
	}
	var out []fyne.CanvasObject
	for _, m := range info.Metrics {
		from, to := r.bounds(time.Now())
		subtitle := T(string(r))
		if r == rangePeriod {
			if m.WindowStart.IsZero() || !m.ResetsAt.After(m.WindowStart) {
				continue
			}
			from, to, subtitle = m.WindowStart, m.ResetsAt, T("reset tarihine kadar")
		}
		samples := history.Samples(accountID, m.ID, from, to)
		title := canvas.NewText(m.Label+" · "+subtitle, nil)
		title.TextSize = 14
		title.TextStyle = fyne.TextStyle{Bold: true}
		legendText := "— " + T("kullanım")
		if r == rangePeriod {
			legendText += "   ┄ " + T("ideal tempo")
		}
		legend := canvas.NewText(legendText, colorMuted)
		legend.TextSize = 11
		head := container.NewBorder(nil, nil, title, legend)
		body := fyne.CanvasObject(newUsageChart(from, to, samples, r == rangePeriod))
		if len(samples) < 2 {
			hint := canvas.NewText(T("Bu aralıkta yeterli ölçüm yok; çizgi sorgular biriktikçe oluşur."), colorMuted)
			hint.TextSize = 11
			body = container.NewVBox(body, container.NewCenter(hint))
		}
		out = append(out, newRoundedCard(container.NewVBox(head, body)))
	}
	return out
}

package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var colorTrack = color.NRGBA{R: 0x8E, G: 0x8E, B: 0x93, A: 0x33}

// coloredProgressBar, doluluk oranına göre rengi değişebilen basit bir ilerleme çubuğudur.
// Standart widget.ProgressBar tema rengini değiştirmeye izin vermediği için özel çizildi.
type coloredProgressBar struct {
	widget.BaseWidget
	value  float64
	color  color.Color
	height float32
	marker float64 // 0-1 arası tempo işareti konumu; <0 ise gizli
}

// newColoredProgressBar, verilen yükseklikte tamamen yuvarlak uçlu bir çubuk oluşturur.
func newColoredProgressBar(height float32) *coloredProgressBar {
	b := &coloredProgressBar{value: 0, color: colorGood, height: height, marker: -1}
	b.ExtendBaseWidget(b)
	return b
}

// SetValue, 0-1 aralığında bir doluluk oranı ve gösterilecek rengi ayarlar.
func (b *coloredProgressBar) SetValue(value float64, col color.Color) {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	b.value = value
	b.color = col
	b.Refresh()
}

// SetMarker, çubuk üzerindeki dikey tempo işaretinin konumunu (0-1) ayarlar.
// Negatif değer işareti gizler.
func (b *coloredProgressBar) SetMarker(value float64) {
	if value > 1 {
		value = 1
	}
	b.marker = value
	b.Refresh()
}

func (b *coloredProgressBar) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(colorTrack)
	fill := canvas.NewRectangle(b.color)
	// Kart zemini renginde kontur, işareti yeşil/turuncu/kırmızı dolgudan ayırır.
	marker := canvas.NewRectangle(colorAccent)
	marker.StrokeColor = theme.Color(colorNameCard)
	marker.StrokeWidth = 2
	marker.CornerRadius = 3

	return &coloredProgressBarRenderer{
		bar:     b,
		bg:      bg,
		fill:    fill,
		marker:  marker,
		objects: []fyne.CanvasObject{bg, fill, marker},
	}
}

type coloredProgressBarRenderer struct {
	bar     *coloredProgressBar
	bg      *canvas.Rectangle
	fill    *canvas.Rectangle
	marker  *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (r *coloredProgressBarRenderer) Layout(size fyne.Size) {
	r.bg.CornerRadius = size.Height / 2
	r.fill.CornerRadius = size.Height / 2
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	w := size.Width * float32(r.bar.value)
	if w > 0 && w < size.Height {
		w = size.Height // küçük değerlerde de yuvarlak bir nokta görünsün
	}
	r.fill.Resize(fyne.NewSize(w, size.Height))
	r.fill.Move(fyne.NewPos(0, 0))

	// Tempo işareti çubuktan biraz taşar ki dolu kısmın üstünde de seçilsin.
	if r.bar.marker < 0 {
		r.marker.Hide()
		return
	}
	const mw, overhang = 9, 9
	x := size.Width*float32(r.bar.marker) - mw/2
	if x < 0 {
		x = 0
	}
	if x > size.Width-mw {
		x = size.Width - mw
	}
	r.marker.Resize(fyne.NewSize(mw, size.Height+2*overhang))
	r.marker.Move(fyne.NewPos(x, -overhang))
	r.marker.Show()
}

func (r *coloredProgressBarRenderer) MinSize() fyne.Size {
	return fyne.NewSize(1, r.bar.height)
}

func (r *coloredProgressBarRenderer) Refresh() {
	r.fill.FillColor = r.bar.color
	r.fill.Refresh()
	r.Layout(r.bar.Size())
}

func (r *coloredProgressBarRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *coloredProgressBarRenderer) Destroy() {}

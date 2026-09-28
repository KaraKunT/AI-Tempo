// Package i18n, arayüz metinlerinin çevirisini sağlar. Kaynak metinler
// Türkçedir ve anahtar olarak kullanılır; İngilizce karşılıkları en.go'dadır.
// Varsayılan tercih sistem dilidir (Auto); desteklenmeyen dillerde İngilizce kullanılır.
package i18n

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Desteklenen diller. Auto, sistem dilini kullanır (Türkçe değilse İngilizce).
const (
	Auto    = "auto"
	English = "en"
	Turkish = "tr"
)

// Languages, ayarlarda gösterilen dil seçenekleridir (kod → kendi dilindeki adı).
// Auto'nun adı T() ile etkin dilde gösterilir.
var Languages = []struct{ Code, Name string }{
	{Auto, "Otomatik (sistem dili)"},
	{English, "English"},
	{Turkish, "Türkçe"},
}

// Resolve, ayardaki tercihi (boş/"auto", "en", "tr") sistem diline göre somut
// dile çevirir. systemLocale "tr-TR" gibi bir BCP 47 etiketidir.
func Resolve(pref, systemLocale string) string {
	switch pref {
	case English, Turkish:
		return pref
	}
	if strings.HasPrefix(strings.ToLower(systemLocale), Turkish) {
		return Turkish
	}
	return English
}

var current atomic.Value

func init() { current.Store(English) }

// Set, etkin dili değiştirir; bilinmeyen kodlarda İngilizceye düşer.
func Set(code string) {
	if code != Turkish {
		code = English
	}
	current.Store(code)
}

// Lang, etkin dil kodudur.
func Lang() string { return current.Load().(string) }

// T, Türkçe kaynak metnin etkin dildeki karşılığını döndürür. Çevirisi
// olmayan metin olduğu gibi döner.
func T(s string) string {
	if Lang() == English {
		if e, ok := en[s]; ok {
			return e
		}
	}
	return s
}

// Tf, çevrilmiş biçim metniyle fmt.Sprintf çağırır.
func Tf(format string, args ...any) string {
	return fmt.Sprintf(T(format), args...)
}

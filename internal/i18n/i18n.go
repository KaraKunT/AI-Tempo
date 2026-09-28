// Package i18n, arayüz metinlerinin çevirisini sağlar. Kaynak metinler
// Türkçedir ve anahtar olarak kullanılır; İngilizce karşılıkları en.go'dadır.
// Varsayılan dil İngilizcedir.
package i18n

import (
	"fmt"
	"sync/atomic"
)

// Desteklenen diller.
const (
	English = "en"
	Turkish = "tr"
)

// Languages, ayarlarda gösterilen dil seçenekleridir (kod → kendi dilindeki adı).
var Languages = []struct{ Code, Name string }{
	{English, "English"},
	{Turkish, "Türkçe"},
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

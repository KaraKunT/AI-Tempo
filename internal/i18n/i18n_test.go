package i18n

import (
	"regexp"
	"testing"
)

// Çeviride biçim belirteçleri (%d, %s, %%, ...) kaynakla aynı sayıda olmalı.
func TestFormatVerbsMatch(t *testing.T) {
	verbs := regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)
	for tr, en := range en {
		if a, b := len(verbs.FindAllString(tr, -1)), len(verbs.FindAllString(en, -1)); a != b {
			t.Errorf("%q → %q: %d / %d belirteç", tr, en, a, b)
		}
	}
}

func TestDefaultEnglish(t *testing.T) {
	Set("")
	if got := T("Yenile"); got != "Refresh" {
		t.Fatalf("varsayılan İngilizce olmalı, %q geldi", got)
	}
	Set(Turkish)
	if got := T("Yenile"); got != "Yenile" {
		t.Fatalf("Türkçe kaynak dönmeli, %q geldi", got)
	}
	Set(English)
}

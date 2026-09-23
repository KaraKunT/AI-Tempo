// Package provider, kota sorgulanabilen AI servislerini (Claude, Cursor,
// ChatGPT) ve kullanım temposu hesabını içerir.
package provider

import (
	"context"
	"time"

	"ai-tempo/internal/config"
)

// UsageMetric, bir hesabın tek bir kota göstergesini temsil eder
// (örn. "Mevcut Oturum (5 Saatlik)" veya "Aylık İstek Limiti").
type UsageMetric struct {
	Label      string
	Subtitle   string
	Percent    float64
	ResetsInfo string
	// TimeProgress, pencerenin başlangıcından bu yana geçen sürenin oranıdır (0-1).
	// Reset zamanına yaklaştıkça 1'e gider. Hesaplanamıyorsa -1 olmalıdır.
	TimeProgress float64
}

// RateLimitInfo, bir hesap için sorgulanmış ve UI'da gösterime hazır kota bilgisidir.
// Her provider kendi metric setini (1 veya daha fazla) döndürür.
type RateLimitInfo struct {
	AccountName string
	Success     bool
	Error       string
	Metrics     []UsageMetric
	// ResetCredits, ChatGPT Codex limit sıfırlama haklarıdır; yalnızca hesapta
	// "Codex sıfırlama haklarını göster" açıksa doldurulur.
	ResetCredits *ResetCredits
}

// ResetCredits, kullanılabilir limit sıfırlama haklarıdır.
type ResetCredits struct {
	Available []ResetCredit // bitiş tarihi en yakın olan başta
	Error     string        // alınamadıysa hata; ana kota yine gösterilir
}

// ResetCredit, tek bir kullanılabilir sıfırlama hakkıdır.
type ResetCredit struct {
	Title     string // örn. "Tam sıfırlama"
	ExpiresAt time.Time
}

// Provider, sorgulanabilir bir AI servisini (Claude, Cursor, ChatGPT, ...) temsil eder.
type Provider interface {
	// ID, config.json'daki "provider" alanıyla eşleşen kısa isimdir (örn. "claude").
	ID() string
	// DisplayName, arayüzde gösterilecek insan-okunur isim.
	DisplayName() string
	// Query, hesabın kota bilgisini sorgular.
	Query(ctx context.Context, account config.Account) RateLimitInfo
}

// providers, ID'ye göre kayıtlı tüm sağlayıcıları tutar.
var providers = map[string]Provider{}

func register(p Provider) {
	providers[p.ID()] = p
}

func Get(id string) Provider {
	if p, ok := providers[id]; ok {
		return p
	}
	// provider belirtilmemişse varsayılan olarak claude kullan (geriye dönük uyumluluk)
	return providers["claude"]
}

// Pace, kullanımın döneme göre geçen süreyle kıyaslanmış temposudur.
type Pace int

const (
	PaceUnknown Pace = iota // zaman bilgisi yok
	PaceSlow                // kullanım, geçen sürenin belirgin şekilde gerisinde
	PaceNormal
	PaceFast // kullanım, geçen sürenin belirgin şekilde önünde; kota erken bitebilir
)

// paceTolerance, "normal" sayılan sapma payıdır (±10 puan).
const paceTolerance = 0.10

// CalcPace, kullanım yüzdesini (0-100) dönemin geçen oranıyla (0-1) karşılaştırır.
func CalcPace(percent, timeProgress float64) Pace {
	if timeProgress < 0 {
		return PaceUnknown
	}
	diff := percent/100 - timeProgress
	switch {
	case diff > paceTolerance:
		return PaceFast
	case diff < -paceTolerance:
		return PaceSlow
	default:
		return PaceNormal
	}
}

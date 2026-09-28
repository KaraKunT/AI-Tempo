// Package provider, kota sorgulanabilen AI servislerini (Claude, Cursor,
// ChatGPT) ve kullanım temposu hesabını içerir.
package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-tempo/internal/config"
	"ai-tempo/internal/i18n"
)

var (
	T  = i18n.T
	Tf = i18n.Tf
)

// UsageMetric, bir hesabın tek bir kota göstergesini temsil eder
// (örn. "Mevcut Oturum (5 Saatlik)" veya "Aylık İstek Limiti").
type UsageMetric struct {
	// ID, göstergenin dilden bağımsız sabit adıdır (grafik ölçümleri bununla saklanır).
	ID         string
	Label      string
	Subtitle   string
	Percent    float64
	ResetsInfo string
	// TimeProgress, pencerenin başlangıcından bu yana geçen sürenin oranıdır (0-1).
	// Reset zamanına yaklaştıkça 1'e gider. Hesaplanamıyorsa -1 olmalıdır.
	TimeProgress float64
	// WindowStart ve ResetsAt, dönemin mutlak başı ve sonudur (grafik için);
	// bilinmiyorsa sıfır kalır.
	WindowStart time.Time
	ResetsAt    time.Time
}

// RateLimitInfo, bir hesap için sorgulanmış ve UI'da gösterime hazır kota bilgisidir.
// Her provider kendi metric setini (1 veya daha fazla) döndürür.
type RateLimitInfo struct {
	AccountName string
	Success     bool
	Error       string
	Metrics     []UsageMetric
	// AuthExpired, hatanın süresi dolmuş/geçersiz oturum anahtarından
	// kaynaklandığını belirtir; arayüz "Anahtarı Güncelle" düğmesi gösterir.
	AuthExpired bool
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

// baseTransport, bağlantıları yeniden kullanır (Cloudflare yerleşik bağlantılara
// daha az şüpheyle bakar).
var baseTransport = http.DefaultTransport.(*http.Transport).Clone()

// resettingTransport, başarısız bir yanıttan sonra boştaki bağlantıları kapatır;
// böylece uyku/ağ değişikliği sonrası "bozulmuş" bir bağlantı yeniden kullanılmaz
// (eskiden bu durum ancak uygulama yeniden başlatılınca düzeliyordu).
type resettingTransport struct{}

func (resettingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := baseTransport.RoundTrip(req)
	if err != nil || resp.StatusCode >= 400 {
		baseTransport.CloseIdleConnections()
	}
	return resp, err
}

// httpClient, tüm sağlayıcıların ortak HTTP istemcisidir.
var httpClient = &http.Client{Timeout: 10 * time.Second, Transport: resettingTransport{}}

// browserUserAgent, isteklerin gerçek bir tarayıcıdan gelmiş gibi görünmesi için kullanılır.
const browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

// isCloudflareBlock, yanıtın oturum hatası değil Cloudflare engeli olup olmadığını tahmin eder.
func isCloudflareBlock(resp *http.Response, body []byte) bool {
	if resp.Header.Get("cf-mitigated") != "" {
		return true
	}
	b := string(body)
	return strings.Contains(b, "Just a moment") || strings.Contains(b, "challenge-platform")
}

// fetch, newReq ile oluşturulan isteği gönderir. Yanıt bir Cloudflare engeliyse
// (genelde geçicidir) 3 sn sonra yeni bir bağlantıyla bir kez daha dener.
// Bağlantı/okuma hatasında info.Error doldurulur ve ok=false döner.
func fetch(ctx context.Context, info *RateLimitInfo, newReq func() (*http.Request, error)) (resp *http.Response, body []byte, ok bool) {
	for attempt := 0; ; attempt++ {
		req, err := newReq()
		if err != nil {
			info.Error = T("İstek hatası")
			return nil, nil, false
		}
		resp, err = httpClient.Do(req)
		if err != nil {
			info.Error = T("API bağlantı hatası")
			return nil, nil, false
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			info.Error = T("Yanıt hatası")
			return nil, nil, false
		}
		if attempt > 0 || resp.StatusCode == http.StatusOK || !isCloudflareBlock(resp, body) {
			return resp, body, true
		}
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			info.Error = T("API bağlantı hatası")
			return nil, nil, false
		}
	}
}

// checkStatus, 200 dışı yanıtları sınıflandırır ve info'yu doldurur; yanıt
// kullanılabilirse true döner. keyValid, anahtarın süresinin dolmadığı yerel
// olarak biliniyorsa true'dur (örn. JWT'nin exp alanı): o durumda 401/403 bir
// oturum sorunu değil, geçici bir erişim engeli sayılır.
func checkStatus(info *RateLimitInfo, resp *http.Response, body []byte, expiredMsg string, keyValid bool) bool {
	switch {
	case resp.StatusCode == http.StatusOK:
		return true
	case isCloudflareBlock(resp, body):
		info.Error = Tf("Cloudflare engeli (HTTP %d), tekrar denenecek", resp.StatusCode)
	case (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && keyValid:
		info.Error = Tf("Erişim geçici olarak reddedildi (HTTP %d), tekrar denenecek", resp.StatusCode) + bodyHint(body)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		info.Error = expiredMsg + fmt.Sprintf(" (HTTP %d)", resp.StatusCode)
		info.AuthExpired = true
	default:
		info.Error = Tf("Sunucu hatası (HTTP %d)", resp.StatusCode) + bodyHint(body)
	}
	return false
}

// bodyHint, hata teşhisi için yanıt gövdesinin kısa, tek satırlık başını döndürür.
func bodyHint(body []byte) string {
	s := strings.Join(strings.Fields(string(body)), " ")
	if s == "" {
		return ""
	}
	if r := []rune(s); len(r) > 80 {
		s = string(r[:80]) + "…"
	}
	return ": " + s
}

// jwtExpiry, JWT biçimindeki bir token'ın exp (bitiş) zamanını döndürür;
// JWT değilse ya da okunamazsa sıfır döner.
func jwtExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}

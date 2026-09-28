package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"ai-tempo/internal/config"
)

func init() {
	register(chatgptProvider{})
}

// chatgptUsageResponse, ChatGPT web arayüzünün /backend-api/wham/usage
// endpoint'inin yanıt gövdesinden ihtiyaç duyduğumuz alanları temsil eder.
type chatgptUsageResponse struct {
	RateLimit struct {
		PrimaryWindow   *chatgptWindow `json:"primary_window"`
		SecondaryWindow *chatgptWindow `json:"secondary_window"`
	} `json:"rate_limit"`
}

type chatgptWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

type chatgptProvider struct{}

func (chatgptProvider) ID() string          { return "chatgpt" }
func (chatgptProvider) DisplayName() string { return "ChatGPT" }

// Query, ChatGPT web oturumundan alınan Bearer access token (session_key)
// ve chatgpt-account-id (organization_id) ile dahili usage endpoint'ini sorgular.
func (chatgptProvider) Query(ctx context.Context, account config.Account) RateLimitInfo {
	info := RateLimitInfo{AccountName: account.Name}

	url := "https://chatgpt.com/backend-api/wham/usage"

	resp, body, ok := fetch(ctx, &info, func() (*http.Request, error) {
		return newChatGPTRequest(ctx, account, url)
	})
	// Token bir JWT: süresi dolmadıysa 401/403 geçici bir engeldir, "süresi dolmuş" değil.
	keyValid := time.Now().Before(jwtExpiry(account.SessionKey))
	if !ok || !checkStatus(&info, resp, body, T("Token süresi dolmuş"), keyValid) {
		return info
	}

	var response chatgptUsageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		info.Error = T("Veri parse hatası")
		return info
	}

	var metrics []UsageMetric
	if w := response.RateLimit.PrimaryWindow; w != nil {
		metrics = append(metrics, UsageMetric{
			ID:           chatgptWindowID(w.LimitWindowSeconds, "primary"),
			Label:        chatgptWindowLabel(w.LimitWindowSeconds, T("Kullanım Limiti")),
			Subtitle:     chatgptWindowSubtitle(w.LimitWindowSeconds),
			Percent:      w.UsedPercent,
			ResetsInfo:   formatResetTimeUnixSeconds(w.ResetAt),
			TimeProgress: timeProgress(time.Unix(w.ResetAt, 0), float64(w.LimitWindowSeconds)),
			WindowStart:  windowStart(time.Unix(w.ResetAt, 0), w.LimitWindowSeconds),
			ResetsAt:     time.Unix(w.ResetAt, 0),
		})
	}
	if w := response.RateLimit.SecondaryWindow; w != nil {
		metrics = append(metrics, UsageMetric{
			ID:           chatgptWindowID(w.LimitWindowSeconds, "secondary"),
			Label:        chatgptWindowLabel(w.LimitWindowSeconds, T("İkincil Limit")),
			Subtitle:     chatgptWindowSubtitle(w.LimitWindowSeconds),
			Percent:      w.UsedPercent,
			ResetsInfo:   formatResetTimeUnixSeconds(w.ResetAt),
			TimeProgress: timeProgress(time.Unix(w.ResetAt, 0), float64(w.LimitWindowSeconds)),
			WindowStart:  windowStart(time.Unix(w.ResetAt, 0), w.LimitWindowSeconds),
			ResetsAt:     time.Unix(w.ResetAt, 0),
		})
	}

	if len(metrics) == 0 {
		info.Error = T("Kota bilgisi bulunamadı")
		return info
	}

	info.Success = true
	info.Metrics = metrics
	if account.ShowResetCredits {
		// Ek bilgi; alınamazsa ana kota yine gösterilir.
		rc, err := fetchChatGPTResetCredits(ctx, account)
		if err != nil {
			rc = ResetCredits{Error: T("Sıfırlama hakları alınamadı")}
		}
		info.ResetCredits = &rc
	}
	return info
}

// newChatGPTRequest, ChatGPT backend-api'si için hesabın token'ıyla GET isteği oluşturur.
func newChatGPTRequest(ctx context.Context, account config.Account, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+account.SessionKey)
	if account.OrganizationID != "" {
		req.Header.Set("chatgpt-account-id", account.OrganizationID)
	}
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", "https://chatgpt.com/codex")
	req.Header.Set("OAI-Language", "tr-TR")
	return req, nil
}

// chatgptGet, ChatGPT backend-api'sine hesabın token'ıyla GET isteği atar.
func chatgptGet(ctx context.Context, account config.Account, path string) ([]byte, error) {
	var info RateLimitInfo
	resp, body, ok := fetch(ctx, &info, func() (*http.Request, error) {
		return newChatGPTRequest(ctx, account, "https://chatgpt.com/backend-api"+path)
	})
	if !ok {
		return nil, fmt.Errorf("%s", info.Error)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}

// fetchChatGPTResetCredits, kullanılabilir Codex limit sıfırlama haklarını sorgular.
func fetchChatGPTResetCredits(ctx context.Context, account config.Account) (ResetCredits, error) {
	body, err := chatgptGet(ctx, account, "/wham/rate-limit-reset-credits")
	if err != nil {
		return ResetCredits{}, err
	}
	return parseChatGPTResetCredits(body)
}

// parseChatGPTResetCredits, yanıttaki kullanılabilir hakları bitiş tarihine göre sıralar.
func parseChatGPTResetCredits(body []byte) (ResetCredits, error) {
	var r struct {
		Credits []struct {
			Status    string `json:"status"`
			Title     string `json:"title"`
			ExpiresAt string `json:"expires_at"`
		} `json:"credits"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return ResetCredits{}, err
	}
	var rc ResetCredits
	for _, c := range r.Credits {
		if c.Status != "available" {
			continue
		}
		t, _ := time.Parse(time.RFC3339Nano, c.ExpiresAt)
		title := c.Title
		if title == "" {
			title = T("Sıfırlama hakkı")
		}
		rc.Available = append(rc.Available, ResetCredit{Title: title, ExpiresAt: t})
	}
	sort.Slice(rc.Available, func(i, j int) bool { return rc.Available[i].ExpiresAt.Before(rc.Available[j].ExpiresAt) })
	return rc, nil
}

// ResetEvent, Codex sıfırlama hakkı geçmişindeki tek bir olaydır.
type ResetEvent struct {
	Kind string // "granted" (kazanıldı) veya "used" (kullanıldı)
	At   time.Time
}

// ResetHistory, sıfırlama hakkı geçmişidir (en yeni olay başta).
type ResetHistory struct {
	Events      []ResetEvent
	WindowStart time.Time // geçmişin kapsadığı dönemin başı
}

// FetchChatGPTResetHistory, Codex sıfırlama hakkı geçmişini sorgular. Otomatik
// yenilemede çağrılmaz; yalnızca kullanıcı istediğinde (düğmeyle) çağrılır.
func FetchChatGPTResetHistory(ctx context.Context, account config.Account) (ResetHistory, error) {
	body, err := chatgptGet(ctx, account, "/wham/rate-limit-reset-credits/history")
	if err != nil {
		return ResetHistory{}, err
	}
	return parseChatGPTResetHistory(body)
}

func parseChatGPTResetHistory(body []byte) (ResetHistory, error) {
	var r struct {
		Events []struct {
			Kind       string `json:"kind"`
			OccurredAt string `json:"occurred_at"`
		} `json:"events"`
		WindowStart string `json:"window_start"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return ResetHistory{}, err
	}
	var h ResetHistory
	h.WindowStart, _ = time.Parse(time.RFC3339Nano, r.WindowStart)
	for _, e := range r.Events {
		t, err := time.Parse(time.RFC3339Nano, e.OccurredAt)
		if err != nil {
			continue
		}
		h.Events = append(h.Events, ResetEvent{Kind: e.Kind, At: t})
	}
	sort.Slice(h.Events, func(i, j int) bool { return h.Events[i].At.After(h.Events[j].At) })
	return h, nil
}

// chatgptWindowLabel, pencere uzunluğu bilinen bir süreyse ona göre başlık
// döndürür (ChatGPT artık yalnızca haftalık pencere gönderebiliyor).
func chatgptWindowLabel(seconds int64, fallback string) string {
	switch seconds {
	case 604800:
		return T("Haftalık Limit")
	case 5 * 3600:
		return T("5 Saatlik Limit")
	}
	return fallback
}

// chatgptWindowID, göstergenin dilden bağımsız sabit adıdır.
func chatgptWindowID(seconds int64, fallback string) string {
	switch seconds {
	case 604800:
		return "weekly"
	case 5 * 3600:
		return "session"
	}
	return fallback
}

// chatgptWindowSubtitle, saniye cinsinden pencere uzunluğunu okunur bir açıklamaya çevirir.
func chatgptWindowSubtitle(seconds int64) string {
	switch {
	case seconds <= 0:
		return ""
	case seconds%604800 == 0:
		weeks := seconds / 604800
		if weeks == 1 {
			return T("Haftalık kullanım")
		}
		return Tf("%d haftalık kullanım", weeks)
	case seconds%86400 == 0:
		days := seconds / 86400
		return Tf("%d günlük kullanım", days)
	case seconds%3600 == 0:
		hours := seconds / 3600
		return Tf("%d saatlik kullanım", hours)
	default:
		return T("Dönemsel kullanım")
	}
}

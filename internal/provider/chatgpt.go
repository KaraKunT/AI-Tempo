package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		info.Error = "İstek hatası"
		return info
	}

	req.Header.Set("Authorization", "Bearer "+account.SessionKey)
	if account.OrganizationID != "" {
		req.Header.Set("chatgpt-account-id", account.OrganizationID)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15)")
	req.Header.Set("Accept", "*/*")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		info.Error = "API bağlantı hatası"
		return info
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		info.Error = "Yanıt hatası"
		return info
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		info.Error = "Token süresi dolmuş"
		return info
	}
	if resp.StatusCode != http.StatusOK {
		info.Error = fmt.Sprintf("API hatası (HTTP %d)", resp.StatusCode)
		return info
	}

	var response chatgptUsageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		info.Error = "Veri parse hatası"
		return info
	}

	var metrics []UsageMetric
	if w := response.RateLimit.PrimaryWindow; w != nil {
		metrics = append(metrics, UsageMetric{
			Label:        "Kullanım Limiti",
			Subtitle:     chatgptWindowSubtitle(w.LimitWindowSeconds),
			Percent:      w.UsedPercent,
			ResetsInfo:   formatResetTimeUnixSeconds(w.ResetAt),
			TimeProgress: timeProgress(time.Unix(w.ResetAt, 0), float64(w.LimitWindowSeconds)),
		})
	}
	if w := response.RateLimit.SecondaryWindow; w != nil {
		metrics = append(metrics, UsageMetric{
			Label:        "İkincil Limit",
			Subtitle:     chatgptWindowSubtitle(w.LimitWindowSeconds),
			Percent:      w.UsedPercent,
			ResetsInfo:   formatResetTimeUnixSeconds(w.ResetAt),
			TimeProgress: timeProgress(time.Unix(w.ResetAt, 0), float64(w.LimitWindowSeconds)),
		})
	}

	if len(metrics) == 0 {
		info.Error = "Kota bilgisi bulunamadı"
		return info
	}

	info.Success = true
	info.Metrics = metrics
	return info
}

// chatgptWindowSubtitle, saniye cinsinden pencere uzunluğunu okunur bir açıklamaya çevirir.
func chatgptWindowSubtitle(seconds int64) string {
	switch {
	case seconds <= 0:
		return ""
	case seconds%604800 == 0:
		weeks := seconds / 604800
		if weeks == 1 {
			return "Haftalık kullanım"
		}
		return fmt.Sprintf("%d haftalık kullanım", weeks)
	case seconds%86400 == 0:
		days := seconds / 86400
		return fmt.Sprintf("%d günlük kullanım", days)
	case seconds%3600 == 0:
		hours := seconds / 3600
		return fmt.Sprintf("%d saatlik kullanım", hours)
	default:
		return "Dönemsel kullanım"
	}
}

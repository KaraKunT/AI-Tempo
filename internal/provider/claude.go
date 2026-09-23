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
	register(claudeProvider{})
}

// claudeUsageResponse, Claude.ai /api/organizations/{id}/usage endpoint'inin
// yanıt gövdesinden ihtiyaç duyduğumuz alanları temsil eder.
type claudeUsageResponse struct {
	FiveHour struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"five_hour"`
	SevenDay struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day"`
}

type claudeProvider struct{}

func (claudeProvider) ID() string          { return "claude" }
func (claudeProvider) DisplayName() string { return "Claude.ai" }

// Query, verilen session cookie ve organization ID ile Claude.ai web
// arayüzünün kullandığı dahili usage endpoint'ini sorgular.
func (claudeProvider) Query(ctx context.Context, account config.Account) RateLimitInfo {
	info := RateLimitInfo{AccountName: account.Name}

	url := fmt.Sprintf("https://claude.ai/api/organizations/%s/usage", account.OrganizationID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		info.Error = "İstek hatası"
		return info
	}

	req.AddCookie(&http.Cookie{
		Name:  "sessionKeyV3",
		Value: account.SessionKey,
	})
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15)")
	req.Header.Set("Accept", "application/json")

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

	if resp.StatusCode != http.StatusOK {
		info.Error = "Session süresi dolmuş"
		return info
	}

	var response claudeUsageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		info.Error = "Veri parse hatası"
		return info
	}

	const fiveHourSeconds = 5 * 3600
	const sevenDaySeconds = 7 * 86400

	sessionResets := ""
	sessionTimeProgress := -1.0
	if response.FiveHour.ResetsAt != "" {
		sessionResets = formatResetTime(response.FiveHour.ResetsAt)
		if t, err := time.Parse(time.RFC3339, response.FiveHour.ResetsAt); err == nil {
			sessionTimeProgress = timeProgress(t, fiveHourSeconds)
		}
	}
	weeklyResets := ""
	weeklyTimeProgress := -1.0
	if response.SevenDay.ResetsAt != "" {
		weeklyResets = formatResetTime(response.SevenDay.ResetsAt)
		if t, err := time.Parse(time.RFC3339, response.SevenDay.ResetsAt); err == nil {
			weeklyTimeProgress = timeProgress(t, sevenDaySeconds)
		}
	}

	info.Success = true
	info.Metrics = []UsageMetric{
		{
			Label:        "Mevcut Oturum",
			Subtitle:     "Son 5 saatlik kullanım",
			Percent:      response.FiveHour.Utilization,
			ResetsInfo:   sessionResets,
			TimeProgress: sessionTimeProgress,
		},
		{
			Label:        "Haftalık Limit",
			Subtitle:     "Son 7 günlük kullanım",
			Percent:      response.SevenDay.Utilization,
			ResetsInfo:   weeklyResets,
			TimeProgress: weeklyTimeProgress,
		},
	}

	return info
}

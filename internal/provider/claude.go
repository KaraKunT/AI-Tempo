package provider

import (
	"context"
	"encoding/json"
	"fmt"
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

	resp, body, ok := fetch(ctx, &info, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.AddCookie(&http.Cookie{Name: "sessionKeyV3", Value: account.SessionKey})
		req.Header.Set("User-Agent", browserUserAgent)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7")
		req.Header.Set("Referer", "https://claude.ai/settings/usage")
		req.Header.Set("anthropic-client-platform", "web_claude_ai")
		return req, nil
	})
	if !ok || !checkStatus(&info, resp, body, T("Session süresi dolmuş"), false) {
		return info
	}

	var response claudeUsageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		info.Error = T("Veri parse hatası")
		return info
	}

	const fiveHourSeconds = 5 * 3600
	const sevenDaySeconds = 7 * 86400

	var sessionEnd, weeklyEnd time.Time
	sessionResets := ""
	sessionTimeProgress := -1.0
	if response.FiveHour.ResetsAt != "" {
		sessionResets = formatResetTime(response.FiveHour.ResetsAt)
		if t, err := time.Parse(time.RFC3339, response.FiveHour.ResetsAt); err == nil {
			sessionEnd = t
			sessionTimeProgress = timeProgress(t, fiveHourSeconds)
		}
	}
	weeklyResets := ""
	weeklyTimeProgress := -1.0
	if response.SevenDay.ResetsAt != "" {
		weeklyResets = formatResetTime(response.SevenDay.ResetsAt)
		if t, err := time.Parse(time.RFC3339, response.SevenDay.ResetsAt); err == nil {
			weeklyEnd = t
			weeklyTimeProgress = timeProgress(t, sevenDaySeconds)
		}
	}

	info.Success = true
	info.Metrics = []UsageMetric{
		{
			ID:           "session",
			Label:        T("Mevcut Oturum"),
			Subtitle:     T("Son 5 saatlik kullanım"),
			Percent:      response.FiveHour.Utilization,
			ResetsInfo:   sessionResets,
			TimeProgress: sessionTimeProgress,
			WindowStart:  windowStart(sessionEnd, fiveHourSeconds),
			ResetsAt:     sessionEnd,
		},
		{
			ID:           "weekly",
			Label:        T("Haftalık Limit"),
			Subtitle:     T("Son 7 günlük kullanım"),
			Percent:      response.SevenDay.Utilization,
			ResetsInfo:   weeklyResets,
			TimeProgress: weeklyTimeProgress,
			WindowStart:  windowStart(weeklyEnd, sevenDaySeconds),
			ResetsAt:     weeklyEnd,
		},
	}

	return info
}

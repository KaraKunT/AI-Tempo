package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-tempo/internal/config"
)

func init() {
	register(claudeProvider{})
}

// claudeWindow, Claude.ai /api/organizations/{id}/usage yanıtındaki tek bir
// kota penceresidir (five_hour, seven_day).
type claudeWindow struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

// claudeUsageResponse, usage yanıtından ihtiyaç duyduğumuz alanlardır.
// Model bazlı haftalık limitler (ör. Fable) yalnızca bazı hesaplarda,
// limits dizisinde kind "weekly_scoped" kaydı olarak gelir.
type claudeUsageResponse struct {
	FiveHour claudeWindow `json:"five_hour"`
	SevenDay claudeWindow `json:"seven_day"`
	Limits   []struct {
		Kind     string  `json:"kind"`
		Percent  float64 `json:"percent"`
		ResetsAt string  `json:"resets_at"`
		Scope    *struct {
			Model *struct {
				DisplayName string `json:"display_name"`
			} `json:"model"`
		} `json:"scope"`
	} `json:"limits"`
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

	metrics, err := parseClaudeUsage(body)
	if err != nil {
		info.Error = T("Veri parse hatası")
		return info
	}
	info.Success = true
	info.Metrics = metrics
	return info
}

// parseClaudeUsage, usage yanıtından 5 saatlik, haftalık ve (hesapta varsa)
// model bazlı haftalık limitleri (ör. Fable) göstergelere çevirir.
func parseClaudeUsage(body []byte) ([]UsageMetric, error) {
	var response claudeUsageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	metric := func(id, label, subtitle string, percent float64, resetsAt string, seconds int64) UsageMetric {
		m := UsageMetric{ID: id, Label: label, Subtitle: subtitle, Percent: percent, TimeProgress: -1}
		if resetsAt != "" {
			m.ResetsInfo = formatResetTime(resetsAt)
			if t, err := time.Parse(time.RFC3339, resetsAt); err == nil {
				m.ResetsAt = t
				m.TimeProgress = timeProgress(t, float64(seconds))
				m.WindowStart = windowStart(t, seconds)
			}
		}
		return m
	}

	const fiveHourSeconds = 5 * 3600
	const sevenDaySeconds = 7 * 86400

	metrics := []UsageMetric{
		metric("session", T("Mevcut Oturum"), T("Son 5 saatlik kullanım"),
			response.FiveHour.Utilization, response.FiveHour.ResetsAt, fiveHourSeconds),
		metric("weekly", T("Haftalık Limit"), T("Son 7 günlük kullanım"),
			response.SevenDay.Utilization, response.SevenDay.ResetsAt, sevenDaySeconds),
	}

	seen := map[string]bool{}
	for _, l := range response.Limits {
		if l.Kind != "weekly_scoped" || l.Scope == nil || l.Scope.Model == nil {
			continue
		}
		name := strings.TrimSpace(l.Scope.Model.DisplayName)
		id := "weekly_" + strings.ToLower(strings.ReplaceAll(name, " ", "_"))
		if name == "" || seen[id] {
			continue
		}
		seen[id] = true
		metrics = append(metrics, metric(id, Tf("%s Limiti", name), T("Son 7 günlük kullanım"),
			l.Percent, l.ResetsAt, sevenDaySeconds))
	}
	return metrics, nil
}

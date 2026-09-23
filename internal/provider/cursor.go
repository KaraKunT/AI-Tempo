package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"ai-tempo/internal/config"
)

func init() {
	register(cursorProvider{})
}

// cursorUsageResponse, Cursor'ın /api/dashboard/get-current-period-usage
// endpoint'inin yanıt gövdesinden ihtiyaç duyduğumuz alanları temsil eder.
type cursorUsageResponse struct {
	BillingCycleStart string `json:"billingCycleStart"` // unix ms, string olarak geliyor
	BillingCycleEnd   string `json:"billingCycleEnd"`   // unix ms, string olarak geliyor
	PlanUsage         struct {
		TotalPercentUsed float64 `json:"totalPercentUsed"`
		AutoPercentUsed  float64 `json:"autoPercentUsed"`
		APIPercentUsed   float64 `json:"apiPercentUsed"`
	} `json:"planUsage"`
}

type cursorProvider struct{}

func (cursorProvider) ID() string          { return "cursor" }
func (cursorProvider) DisplayName() string { return "Cursor" }

// Query, WorkosCursorSessionToken cookie'si ile Cursor dashboard'unun
// kullandığı dahili "get-current-period-usage" endpoint'ini sorgular.
func (cursorProvider) Query(ctx context.Context, account config.Account) RateLimitInfo {
	info := RateLimitInfo{AccountName: account.Name}

	url := "https://cursor.com/api/dashboard/get-current-period-usage"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString("{}"))
	if err != nil {
		info.Error = "İstek hatası"
		return info
	}

	req.AddCookie(&http.Cookie{
		Name:  "WorkosCursorSessionToken",
		Value: account.SessionKey,
	})
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15)")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://cursor.com")
	req.Header.Set("Referer", "https://cursor.com/dashboard/spending")

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

	var response cursorUsageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		info.Error = "Veri parse hatası"
		return info
	}

	resetsInfo := ""
	cycleProgress := -1.0
	endMs, endErr := parseUnixMilliString(response.BillingCycleEnd)
	if endErr == nil {
		resetsInfo = formatResetTimeUnixMilli(endMs)
		if startMs, startErr := parseUnixMilliString(response.BillingCycleStart); startErr == nil && endMs > startMs {
			windowSeconds := float64(endMs-startMs) / 1000
			cycleProgress = timeProgress(time.UnixMilli(endMs), windowSeconds)
		}
	}

	info.Success = true
	info.Metrics = []UsageMetric{
		{
			Label:        "Toplam Kullanım",
			Subtitle:     "Fatura dönemi (dahil + bonus)",
			Percent:      response.PlanUsage.TotalPercentUsed,
			ResetsInfo:   resetsInfo,
			TimeProgress: cycleProgress,
		},
		{
			Label:        "Otomatik Model",
			Subtitle:     "Auto model kullanımı",
			Percent:      response.PlanUsage.AutoPercentUsed,
			ResetsInfo:   resetsInfo,
			TimeProgress: cycleProgress,
		},
		{
			Label:        "API Kullanımı",
			Subtitle:     "İsimli model / API kullanımı",
			Percent:      response.PlanUsage.APIPercentUsed,
			ResetsInfo:   resetsInfo,
			TimeProgress: cycleProgress,
		},
	}

	return info
}

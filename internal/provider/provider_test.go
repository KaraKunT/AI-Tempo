package provider

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fakeJWT(exp int64) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + itoa(exp) + `}`))
	return "eyJhbGciOiJSUzI1NiJ9." + payload + ".sig"
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestJWTExpiry(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	if got := jwtExpiry(fakeJWT(exp)); got.Unix() != exp {
		t.Fatalf("exp okunamadı: %v", got)
	}
	if !jwtExpiry("sessionkey-not-a-jwt").IsZero() {
		t.Fatal("JWT olmayan anahtar sıfır dönmeli")
	}
}

func TestCheckStatus(t *testing.T) {
	resp := func(code int) *http.Response { return &http.Response{StatusCode: code, Header: http.Header{}} }

	var info RateLimitInfo
	if checkStatus(&info, resp(403), []byte(`{"detail":"x"}`), "Token süresi dolmuş", true) || info.AuthExpired {
		t.Fatalf("geçerli token'la 403 süresi dolmuş sayılmamalı: %+v", info)
	}

	info = RateLimitInfo{}
	if checkStatus(&info, resp(401), nil, "Token süresi dolmuş", false) || !info.AuthExpired {
		t.Fatalf("süresi bilinmeyen anahtarla 401 AuthExpired olmalı: %+v", info)
	}

	info = RateLimitInfo{}
	if checkStatus(&info, resp(403), []byte("<title>Just a moment...</title>"), "x", false) || info.AuthExpired {
		t.Fatalf("Cloudflare sayfası oturum hatası sayılmamalı: %+v", info)
	}

	info = RateLimitInfo{}
	if checkStatus(&info, resp(307), nil, "Session süresi dolmuş", false) || !info.AuthExpired {
		t.Fatalf("giriş sayfasına yönlendirme oturum hatası sayılmalı: %+v", info)
	}

	info = RateLimitInfo{}
	if !checkStatus(&info, resp(200), nil, "x", false) {
		t.Fatal("200 kabul edilmeli")
	}
}

func TestParseClaudeUsageModelLimits(t *testing.T) {
	// Gerçek yanıttan kısaltılmış örnek (2026-11-10).
	body := []byte(`{
		"five_hour": {"utilization": 22.0, "resets_at": "2026-11-10T02:50:00.476557+00:00"},
		"seven_day": {"utilization": 28.0, "resets_at": "2026-11-13T05:00:00.476577+00:00"},
		"seven_day_opus": null,
		"seven_day_breakdown": null,
		"limits": [
			{"kind": "session", "group": "session", "percent": 22, "resets_at": "2026-11-10T02:50:00.476557+00:00", "scope": null},
			{"kind": "weekly_all", "group": "weekly", "percent": 28, "resets_at": "2026-11-13T05:00:00.476577+00:00", "scope": null},
			{"kind": "weekly_scoped", "group": "weekly", "percent": 21, "resets_at": "2026-11-13T05:00:00.476894+00:00",
			 "scope": {"model": {"id": null, "display_name": "Fable"}, "surface": null}}
		]
	}`)
	metrics, err := parseClaudeUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 3 || metrics[0].Percent != 22 || metrics[1].Percent != 28 {
		t.Fatalf("3 gösterge beklendi: %+v", metrics)
	}
	f := metrics[2]
	if f.ID != "weekly_fable" || f.Percent != 21 || !strings.HasPrefix(f.Label, "Fable ") || f.ResetsAt.IsZero() {
		t.Fatalf("Fable limiti hatalı: %+v", f)
	}

	// Fable limiti olmayan hesap: yalnızca iki gösterge.
	metrics, err = parseClaudeUsage([]byte(`{"five_hour":{"utilization":0},"seven_day":{"utilization":5},"limits":[]}`))
	if err != nil || len(metrics) != 2 {
		t.Fatalf("2 gösterge beklendi: %+v %v", metrics, err)
	}
}

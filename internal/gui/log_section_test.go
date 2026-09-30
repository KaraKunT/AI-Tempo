package gui

import (
	"fmt"
	"testing"
)

func TestParseLogMetrics(t *testing.T) {
	got := parseLogMetrics("Mevcut Oturum %5 · Haftalık Limit %70")
	if len(got) != 2 || got[0].Metric != "Mevcut Oturum" || got[0].Percent != 5 || got[1].Percent != 70 {
		t.Fatalf("Türkçe biçim: %+v", got)
	}
	got = parseLogMetrics("Current Session 21% · Weekly Limit 72%")
	if len(got) != 2 || got[1].Metric != "Weekly Limit" || got[1].Percent != 72 {
		t.Fatalf("İngilizce biçim: %+v", got)
	}
	if got := parseLogMetrics("Cloudflare engeli (HTTP 403), tekrar denenecek"); got != nil {
		t.Fatalf("hata mesajı çözülmemeli: %+v", got)
	}
}

func TestNormalizeKey(t *testing.T) {
	if got := normalizeKey("  Bearer eyJabc\n.def \t.ghi\n"); got != "eyJabc.def.ghi" {
		t.Fatalf("normalizeKey = %q", got)
	}
}

func TestKeySummaryTruncated(t *testing.T) {
	if text, _ := keySummary("eyJhbGciOiJSUzI1NiIs…nbGUtb2F1dGgyfDEw"); text[:3] != "⚠" {
		t.Fatalf("kesilmiş anahtar uyarı vermeli: %q", text)
	}
	if text, _ := keySummary("eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9"); text[:3] != "✓" {
		t.Fatalf("sağlam anahtar ✓ ile başlamalı: %q", text)
	}
}

func TestPageWindow(t *testing.T) {
	cases := []struct {
		page, pages int
		want        string
	}{
		{0, 1, "[0]"},
		{0, 3, "[0 1 2]"},
		{0, 10, "[0 1 -1 9]"},
		{5, 10, "[0 -1 4 5 6 -1 9]"},
		{9, 10, "[0 -1 8 9]"},
		{2, 10, "[0 1 2 3 -1 9]"},
	}
	for _, c := range cases {
		if got := fmt.Sprint(pageWindow(c.page, c.pages)); got != c.want {
			t.Errorf("pageWindow(%d, %d) = %s, want %s", c.page, c.pages, got, c.want)
		}
	}
}

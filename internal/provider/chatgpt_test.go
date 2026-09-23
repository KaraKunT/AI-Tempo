package provider

import "testing"

func TestParseChatGPTResetCredits(t *testing.T) {
	body := `{
	  "credits": [
	    {"status": "available", "title": "Tam sıfırlama", "expires_at": "2026-11-05T10:00:00Z"},
	    {"status": "available", "title": "Tam sıfırlama", "expires_at": "2026-10-22T21:08:34.244176Z"},
	    {"status": "redeemed",  "title": "Tam sıfırlama", "expires_at": "2026-09-01T10:00:00Z"}
	  ],
	  "available_count": 2
	}`
	rc, err := parseChatGPTResetCredits([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	// Kullanılmış hak yok sayılır, bitişi en yakın olan başta gelir.
	if len(rc.Available) != 2 || rc.Available[0].ExpiresAt.Month() != 10 || rc.Available[0].Title != "Tam sıfırlama" {
		t.Fatalf("beklenmeyen haklar: %+v", rc.Available)
	}
}

func TestParseChatGPTResetCreditsNone(t *testing.T) {
	rc, err := parseChatGPTResetCredits([]byte(`{"credits": [], "available_count": 0}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(rc.Available) != 0 {
		t.Fatalf("hak olmamalı: %+v", rc.Available)
	}
}

func TestParseChatGPTResetHistory(t *testing.T) {
	body := `{
	  "events": [
	    {"kind": "used",    "occurred_at": "2026-09-05T00:58:21.863089Z"},
	    {"kind": "granted", "occurred_at": "2026-09-22T21:08:34.244176Z"},
	    {"kind": "granted", "occurred_at": "bozuk-tarih"}
	  ],
	  "window_start": "2026-08-24T12:04:53.842540Z"
	}`
	h, err := parseChatGPTResetHistory([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	// Bozuk tarihli olay atlanır, kalanlar en yeni başta sıralanır.
	if len(h.Events) != 2 || h.Events[0].Kind != "granted" || h.Events[1].Kind != "used" {
		t.Fatalf("beklenmeyen olaylar: %+v", h.Events)
	}
	if h.WindowStart.IsZero() {
		t.Fatal("window_start okunmadı")
	}
}

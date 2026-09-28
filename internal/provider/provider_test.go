package provider

import (
	"encoding/base64"
	"net/http"
	"strconv"
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
	if !checkStatus(&info, resp(200), nil, "x", false) {
		t.Fatal("200 kabul edilmeli")
	}
}

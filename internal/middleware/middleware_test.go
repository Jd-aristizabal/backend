package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestProxyTrust(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "1.2.3.4:123"
	r.Header.Set("X-Forwarded-For", "9.8.7.6")
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/16")}
	if ClientIP(r, trusted) != "1.2.3.4" {
		t.Fatal("header falsificado aceptado")
	}
	r.RemoteAddr = "10.0.1.3:123"
	r.Header.Set("X-Forwarded-For", "9.8.7.6, 1.2.3.4")
	if ClientIP(r, trusted) != "1.2.3.4" {
		t.Fatal("cadena proxy incorrecta")
	}
}
func TestCORS(t *testing.T) {
	handler := Base(slog.New(slog.NewJSONHandler(io.Discard, nil)), []string{"https://jd-aristizabal.vercel.app"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	for _, tt := range []struct {
		origin string
		status int
	}{{"https://evil.example", 403}, {"https://jd-aristizabal.vercel.app", 204}} {
		r := httptest.NewRequest("OPTIONS", "/api/v1/chat", nil)
		r.Header.Set("Origin", tt.origin)
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", "X-Client-ID, Content-Type")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tt.status {
			t.Fatal(w.Code)
		}
	}
}

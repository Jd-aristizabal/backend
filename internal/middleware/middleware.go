package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/Jd-aristizabal/backend/internal/shared"
	"github.com/Jd-aristizabal/backend/internal/transport"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type key int

const clientKey key = 1

func Client(r *http.Request) uuid.UUID { return r.Context().Value(clientKey).(uuid.UUID) }
func Identity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values("X-Client-ID")
		if len(values) != 1 {
			transport.Error(w, shared.Invalid("X-Client-ID es obligatorio"))
			return
		}
		id, err := transport.UUID(values[0])
		if err != nil {
			transport.Error(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientKey, id)))
	})
}
func Base(logger *slog.Logger, origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			id := uuid.NewString()
			w.Header().Set("X-Request-ID", id)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				if recover() != nil {
					transport.Error(ww, &shared.Error{Status: 500, Code: "INTERNAL_ERROR", Message: "No se pudo completar la solicitud"})
					logger.Error("request_panic", "request_id", id)
				}
				status := ww.Status()
				if status == 0 {
					status = 200
				}
				logger.Info("request", "request_id", id, "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(start).Milliseconds())
			}()
			origin := r.Header.Get("Origin")
			ww.Header().Add("Vary", "Origin")
			if origin != "" {
				if !allowed[origin] {
					transport.Error(ww, &shared.Error{Status: 403, Code: "ORIGIN_FORBIDDEN", Message: "Origen no permitido"})
					return
				}
				ww.Header().Set("Access-Control-Allow-Origin", origin)
				ww.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
			}
			if r.Method == http.MethodOptions {
				if origin == "" {
					transport.Error(ww, &shared.Error{Status: 400, Code: "INVALID_PREFLIGHT", Message: "Origin es obligatorio"})
					return
				}
				method := r.Header.Get("Access-Control-Request-Method")
				if method != "GET" && method != "POST" && method != "PATCH" && method != "DELETE" {
					transport.Error(ww, &shared.Error{Status: 403, Code: "ORIGIN_FORBIDDEN", Message: "Método no permitido"})
					return
				}
				for _, h := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
					h = strings.TrimSpace(strings.ToLower(h))
					if h != "" && h != "content-type" && h != "x-client-id" {
						transport.Error(ww, &shared.Error{Status: 403, Code: "ORIGIN_FORBIDDEN", Message: "Header no permitido"})
						return
					}
				}
				ww.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				ww.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Client-ID")
				ww.Header().Set("Access-Control-Max-Age", "600")
				ww.WriteHeader(204)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
			defer cancel()
			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}
func ClientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	remote = remote.Unmap()
	isTrusted := func(ip netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(ip) {
				return true
			}
		}
		return false
	}
	if isTrusted(remote) {
		chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if len(chain) <= 10 {
			for i := len(chain) - 1; i >= 0; i-- {
				ip, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
				if err != nil {
					break
				}
				remote = ip.Unmap()
				if !isTrusted(remote) {
					break
				}
			}
		}
	}
	return remote.String()
}
func IPKey(r *http.Request, trusted []netip.Prefix) string {
	sum := sha256.Sum256([]byte(ClientIP(r, trusted)))
	return fmt.Sprintf("%x", sum[:])
}

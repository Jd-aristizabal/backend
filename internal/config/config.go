package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env, Port, DatabaseURL, AIProvider, AIKey, AIModel, AIBaseURL string
	Origins                                                       []string
	TrustedProxies                                                []netip.Prefix
	AITimeout                                                     time.Duration
	MaxTokens, ChatDailyLimit                                     int
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func Load() (Config, error) {
	c := Config{Env: value("APP_ENV", "development"), Port: value("PORT", "8080"), DatabaseURL: os.Getenv("DATABASE_URL"), AIProvider: value("AI_PROVIDER", "disabled"), AIKey: os.Getenv("AI_API_KEY"), AIModel: os.Getenv("AI_MODEL"), AIBaseURL: value("AI_BASE_URL", "https://api.openai.com/v1"), AITimeout: 20 * time.Second, MaxTokens: 400, ChatDailyLimit: 200}
	if c.Env != "production" && c.Env != "development" && c.Env != "test" {
		return c, fmt.Errorf("APP_ENV inválido")
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("PORT inválido")
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL es obligatorio")
	}
	db, err := url.Parse(c.DatabaseURL)
	if err != nil || (db.Scheme != "postgres" && db.Scheme != "postgresql") || db.Host == "" {
		return c, fmt.Errorf("DATABASE_URL inválido")
	}
	if c.Env == "production" && db.Query().Get("sslmode") != "verify-full" {
		return c, fmt.Errorf("producción requiere sslmode=verify-full")
	}
	for _, origin := range strings.Split(value("CORS_ALLOWED_ORIGINS", "https://jd-aristizabal.vercel.app,http://localhost:5173"), ",") {
		origin = strings.TrimSpace(origin)
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return c, fmt.Errorf("origen CORS inválido")
		}
		if c.Env == "production" && origin != "https://jd-aristizabal.vercel.app" {
			return c, fmt.Errorf("origen de producción no permitido")
		}
		c.Origins = append(c.Origins, origin)
	}
	for _, raw := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		if raw == "" {
			continue
		}
		p, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return c, fmt.Errorf("TRUSTED_PROXY_CIDRS inválido")
		}
		c.TrustedProxies = append(c.TrustedProxies, p)
	}
	if v := os.Getenv("CHAT_DAILY_LIMIT"); v != "" {
		c.ChatDailyLimit, err = strconv.Atoi(v)
		if err != nil || c.ChatDailyLimit < 1 || c.ChatDailyLimit > 10000 {
			return c, fmt.Errorf("CHAT_DAILY_LIMIT inválido")
		}
	}
	if c.AIProvider != "disabled" && c.AIProvider != "openai" && c.AIProvider != "openai-compatible" {
		return c, fmt.Errorf("AI_PROVIDER no soportado")
	}
	if c.AIProvider != "disabled" {
		if c.AIKey == "" || c.AIModel == "" {
			return c, fmt.Errorf("AI_API_KEY y AI_MODEL son obligatorios")
		}
		u, err := url.Parse(c.AIBaseURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return c, fmt.Errorf("AI_BASE_URL requiere HTTPS")
		}
		if c.AIProvider == "openai" && c.AIBaseURL != "https://api.openai.com/v1" {
			return c, fmt.Errorf("usa openai-compatible para otro endpoint")
		}
	}
	return c, nil
}

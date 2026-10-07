package config

import "testing"

func TestProductionConfig(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://app:password@database:5432/portfolio?sslmode=verify-full")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://jd-aristizabal.vercel.app")
	t.Setenv("AI_PROVIDER", "disabled")
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.42.1.0/24,10.42.2.0/24")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "postgres://app:password@database:5432/portfolio?sslmode=disable")
	if _, err := Load(); err == nil {
		t.Fatal("TLS opcional en producción")
	}
	t.Setenv("DATABASE_URL", "postgres://app:password@database:5432/portfolio?sslmode=verify-full")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")
	if _, err := Load(); err == nil {
		t.Fatal("CORS abierto en producción")
	}
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://jd-aristizabal.vercel.app")
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("AI_API_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("IA sin credencial aceptada")
	}
}

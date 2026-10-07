package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/Jd-aristizabal/backend/internal/config"
	"github.com/Jd-aristizabal/backend/internal/database"
	httpapi "github.com/Jd-aristizabal/backend/internal/http"
	"github.com/Jd-aristizabal/backend/internal/modules/chatbot"
)

func run(log *slog.Logger) error {
	c, err := config.Load()
	if err != nil {
		log.Error("config_invalid", "reason", err.Error())
		return errors.New("configuración inválida")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	db, err := database.Open(startup, c.DatabaseURL)
	cancel()
	if err != nil {
		return errors.New("no se pudo conectar a PostgreSQL")
	}
	defer db.Close()
	router, limiter, err := httpapi.New(c, db, log, chatbot.NewProvider(c))
	if err != nil {
		return errors.New("no se pudo iniciar la API")
	}
	go limiter.Cleanup(ctx)
	server := &http.Server{Addr: ":" + c.Port, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	done := make(chan error, 1)
	go func() {
		log.Info("server_started", "port", c.Port, "environment", c.Env, "ai_provider", c.AIProvider)
		done <- server.ListenAndServe()
	}()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("servidor HTTP detenido")
		}
		return nil
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return errors.New("apagado incompleto")
	}
	log.Info("server_stopped")
	return nil
}
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("startup_failed", "reason", err.Error())
		os.Exit(1)
	}
}

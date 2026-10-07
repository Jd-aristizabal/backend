package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Jd-aristizabal/backend/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		return fmt.Errorf("uso: migrate up|down (down revierte una versión)")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL es obligatorio")
	}
	source, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return fmt.Errorf("no se pudieron leer las migraciones")
	}
	url = strings.Replace(url, "postgresql://", "pgx5://", 1)
	url = strings.Replace(url, "postgres://", "pgx5://", 1)
	m, err := migrate.NewWithSourceInstance("iofs", source, url)
	if err != nil {
		return fmt.Errorf("no se pudo conectar para migrar")
	}
	defer m.Close()
	if os.Args[1] == "up" {
		err = m.Up()
	} else {
		err = m.Steps(-1)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("migración fallida; revisar versión y estado dirty")
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Migraciones aplicadas")
}

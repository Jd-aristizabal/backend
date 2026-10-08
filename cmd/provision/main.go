package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Este comando se ejecuta como tarea administrativa antes de iniciar ECS.
func run() error {
	password := os.Getenv("APP_DB_PASSWORD")
	if len(password) < 24 || len(password) > 128 || strings.ContainsAny(password, "\x00\n\r") {
		return fmt.Errorf("APP_DB_PASSWORD debe contener entre 24 y 128 caracteres")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("conexión administrativa fallida")
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transacción fallida")
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='backend_api')").Scan(&exists); err != nil {
		return fmt.Errorf("consulta de rol fallida")
	}
	quoted := "'" + strings.ReplaceAll(password, "'", "''") + "'"
	sql := "CREATE ROLE backend_api LOGIN PASSWORD " + quoted + " NOSUPERUSER NOCREATEDB NOCREATEROLE"
	if exists {
		sql = "ALTER ROLE backend_api LOGIN PASSWORD " + quoted + " NOSUPERUSER NOCREATEDB NOCREATEROLE"
	}
	if _, err = tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("creación de rol fallida")
	}
	var db string
	if err = tx.QueryRow(ctx, "SELECT current_database()").Scan(&db); err != nil {
		return fmt.Errorf("consulta de base fallida")
	}
	statements := []string{
		"REVOKE CREATE ON SCHEMA public FROM PUBLIC",
		"GRANT CONNECT ON DATABASE " + pgx.Identifier{db}.Sanitize() + " TO backend_api",
		"GRANT USAGE ON SCHEMA public TO backend_api",
		"GRANT SELECT,INSERT,UPDATE,DELETE ON focus_tasks,focus_sessions,pixel_sprint_scores,arcade_scores,chat_conversations,chat_messages,rate_limit_buckets TO backend_api",
		"GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO backend_api",
	}
	for _, sql := range statements {
		if _, err = tx.Exec(ctx, sql); err != nil {
			return fmt.Errorf("asignación de permisos fallida; aplica migraciones antes")
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmación administrativa fallida")
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Rol de aplicación preparado")
}

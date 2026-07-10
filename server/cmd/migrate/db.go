package main

import (
	"database/sql"

	// pgx provides a database/sql compatible driver via this import
	_ "github.com/jackc/pgx/v5/stdlib"
)

// openDB opens a *sql.DB connection using pgx's stdlib adapter.
// goose requires *sql.DB (not pgx.Conn), so we bridge via pgx/v5/stdlib.
func openDB(dsn string) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
}

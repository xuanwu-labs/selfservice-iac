// Package main is the Aether DB migration runner.
// Uses goose with //go:embed for self-contained binary.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func main() {
	// Load config (same four-layer priority as server: flag > env > yaml > default).
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(configPath, "migrate")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	dbDSN := cfg.Data.Database.DSN()
	if dbDSN == "" {
		fmt.Fprintln(os.Stderr, "database not configured")
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:   "aether-migrate",
		Short: "Aether database migration runner (goose)",
	}

	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withGoose(cmd.Context(), dbDSN, func(g *goose.Provider) error {
				results, err := g.Up(cmd.Context())
				if err != nil {
					return err
				}
				for _, r := range results {
					fmt.Printf("  Applied: %s (v%d)\n", r.Source.Path, r.Source.Version)
				}
				return nil
			})
		},
	}

	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Roll back the last migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withGoose(cmd.Context(), dbDSN, func(g *goose.Provider) error {
				result, err := g.Down(cmd.Context())
				if err != nil {
					return err
				}
				if result != nil {
					fmt.Printf("  Rolled back: %s (v%d)\n", result.Source.Path, result.Source.Version)
				}
				return nil
			})
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show migration status (full version history)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withGoose(cmd.Context(), dbDSN, func(g *goose.Provider) error {
				statuses, err := g.Status(cmd.Context())
				if err != nil {
					return err
				}
				fmt.Printf("%-10s %-12s %-20s %s\n", "VERSION", "STATE", "APPLIED_AT", "FILENAME")
				for _, s := range statuses {
					applied := "pending"
					if !s.AppliedAt.IsZero() {
						applied = s.AppliedAt.Format("2006-01-02 15:04:05")
					}
					fmt.Printf("%-10d %-12s %-20s %s\n",
						s.Source.Version, s.State, applied, filepath.Base(s.Source.Path))
				}
				return nil
			})
		},
	}

	redoCmd := &cobra.Command{
		Use:   "redo",
		Short: "Roll back then re-apply the last migration (idempotency check)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withGoose(cmd.Context(), dbDSN, func(g *goose.Provider) error {
				if _, err := g.Down(cmd.Context()); err != nil {
					return err
				}
				_, err := g.Up(cmd.Context())
				return err
			})
		},
	}

	rootCmd.AddCommand(upCmd, downCmd, statusCmd, redoCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// withGoose connects to PG (via database/sql for goose), creates provider with embedded migrations.
func withGoose(ctx context.Context, dsn string, fn func(*goose.Provider) error) error {
	// goose requires *sql.DB (standard library), so use lib/pq compatible DSN
	db, err := openDB(dsn)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Create a sub-FS that strips the "migrations/" prefix
	migrationFS, err := fs.Sub(embedMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("embed sub-FS: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrationFS,
	)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}

	return fn(provider)
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/abishekkumar/claude-memory/cli/internal/config"
	"github.com/abishekkumar/claude-memory/cli/internal/db"
	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func MigrateCmd() *cli.Command {
	return &cli.Command{
		Name:  "migrate",
		Usage: "Apply pending SQL migrations from the migrations/ directory",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Value: "./migrations",
				Usage: "Path to migrations directory"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.Load()
			if err != nil {
				output.Fail(err.Error())
				return cli.Exit("", 1)
			}
			return runMigrate(c.Context, cfg, c.String("dir"))
		},
	}
}

func runMigrate(ctx context.Context, cfg *config.Config, migrationsDir string) error {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		output.Fail("Cannot connect to database: " + err.Error())
		return cli.Exit("", 1)
	}
	defer pool.Close()

	if err := db.Exec(ctx, pool, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename text PRIMARY KEY,
			applied_at timestamptz DEFAULT now()
		)`); err != nil {
		output.Fail("Cannot create migrations table: " + err.Error())
		return cli.Exit("", 1)
	}

	appliedRows, _ := db.Fetch(ctx, pool, "SELECT filename FROM schema_migrations")
	applied := map[string]bool{}
	for _, r := range appliedRows {
		applied[fmt.Sprint(r["filename"])] = true
	}

	entries, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil || len(entries) == 0 {
		output.Info("No migration files found in " + migrationsDir)
		return nil
	}
	sort.Strings(entries)

	var pending []string
	for _, f := range entries {
		if !applied[filepath.Base(f)] {
			pending = append(pending, f)
		}
	}

	if len(pending) == 0 {
		output.Info("All migrations already applied.")
		return nil
	}

	for _, f := range pending {
		fmt.Printf("  Applying %s...\n", filepath.Base(f))
		sql, err := os.ReadFile(f)
		if err != nil {
			output.Fail("Read failed: " + err.Error())
			return cli.Exit("", 1)
		}
		if err := db.Exec(ctx, pool, string(sql)); err != nil {
			output.Fail("Migration failed: " + err.Error())
			return cli.Exit("", 1)
		}
		if err := db.Exec(ctx, pool,
			"INSERT INTO schema_migrations (filename) VALUES ($1)", filepath.Base(f)); err != nil {
			output.Fail("Tracking failed: " + err.Error())
			return cli.Exit("", 1)
		}
		output.OK(filepath.Base(f) + " applied")
	}
	output.OK(fmt.Sprintf("%d migration(s) applied.", len(pending)))
	return nil
}

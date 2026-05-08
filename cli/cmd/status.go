package cmd

import (
	"context"
	"fmt"

	"github.com/abishekkumar/claude-memory/cli/internal/config"
	"github.com/abishekkumar/claude-memory/cli/internal/db"
	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func StatusCmd() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show operational status — memory count, DB size, last write",
		Action: func(c *cli.Context) error {
			cfg, err := config.Load()
			if err != nil {
				output.Fail(err.Error())
				return cli.Exit("", 1)
			}
			return runStatus(c.Context, cfg)
		},
	}
}

func runStatus(ctx context.Context, cfg *config.Config) error {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		output.Fail("Cannot connect to database: " + err.Error())
		return cli.Exit("", 1)
	}
	defer pool.Close()

	queries := []struct {
		label string
		query string
	}{
		{"Active memories", "SELECT count(*)::text FROM memory_events WHERE metadata->>'status' = 'active'"},
		{"Database size", "SELECT pg_size_pretty(pg_database_size(current_database()))"},
		{"Last write", "SELECT coalesce(max(created_at)::text, 'never') FROM memory_events"},
		{"Active projects", `SELECT count(DISTINCT coalesce(metadata->>'project', metadata->>'workspace_path'))::text
		                     FROM memory_events WHERE metadata->>'project' IS NOT NULL AND metadata->>'status' = 'active'`},
	}

	output.Bold("\nClaude Memory — Status\n")
	t := output.Table("Metric", "Value")
	for _, q := range queries {
		val, err := db.FetchVal(ctx, pool, q.query)
		s := "—"
		if err == nil && val != nil {
			s = fmt.Sprint(val)
			if len(s) > 19 {
				s = s[:19] // trim timestamp
			}
		}
		t.AppendRow(table.Row{q.label, s})
	}
	t.Render()
	fmt.Println()
	return nil
}

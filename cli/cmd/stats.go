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

func StatsCmd() *cli.Command {
	return &cli.Command{
		Name:  "stats",
		Usage: "Analytics dashboard — breakdown by type, scope, topic, project, activity",
		Action: func(c *cli.Context) error {
			cfg, err := config.Load()
			if err != nil {
				output.Fail(err.Error())
				return cli.Exit("", 1)
			}
			return runStats(c.Context, cfg)
		},
	}
}

func runStats(ctx context.Context, cfg *config.Config) error {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		output.Fail("Cannot connect to database: " + err.Error())
		return cli.Exit("", 1)
	}
	defer pool.Close()

	output.Bold("\nClaude Memory — Statistics\n")

	printTable := func(title string, headers []string, query string, args ...any) {
		rows, err := db.Fetch(ctx, pool, query, args...)
		t := output.Table(headers...)
		if err == nil {
			for _, r := range rows {
				row := make(table.Row, len(headers))
				i := 0
				for _, v := range r {
					if i < len(row) {
						row[i] = fmt.Sprint(v)
						i++
					}
				}
				t.AppendRow(row)
			}
		}
		fmt.Printf("  %s\n", title)
		t.Render()
		fmt.Println()
	}

	printTable("By type", []string{"Type", "Count"},
		`SELECT memory_type, count(*) AS n FROM memory_events
		 WHERE metadata->>'status' = 'active'
		 GROUP BY memory_type ORDER BY n DESC`)

	printTable("By scope", []string{"Scope", "Count"},
		`SELECT scope, count(*) AS n FROM memory_events
		 WHERE metadata->>'status' = 'active'
		 GROUP BY scope ORDER BY n DESC`)

	printTable("Top topics", []string{"Topic", "Count"},
		`SELECT metadata->>'topic' AS topic, count(*) AS n
		 FROM memory_events
		 WHERE metadata->>'topic' IS NOT NULL AND metadata->>'status' = 'active'
		 GROUP BY topic ORDER BY n DESC LIMIT 10`)

	printTable("Top projects (basename)", []string{"Project", "Count"},
		`SELECT coalesce(metadata->>'project', metadata->>'workspace_path') AS project,
		        count(*) AS n
		 FROM memory_events
		 WHERE coalesce(metadata->>'project', metadata->>'workspace_path') IS NOT NULL
		   AND metadata->>'status' = 'active'
		 GROUP BY project ORDER BY n DESC LIMIT 10`)

	printTable("Activity (14d)", []string{"Date", "Writes"},
		`SELECT date_trunc('day', created_at)::date::text AS day, count(*) AS n
		 FROM memory_events
		 WHERE created_at > now() - interval '14 days'
		 GROUP BY day ORDER BY day DESC`)

	return nil
}

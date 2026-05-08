package cmd

import (
	"context"
	"fmt"

	"github.com/abishekkumar/claude-memory/cli/internal/config"
	"github.com/abishekkumar/claude-memory/cli/internal/db"
	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func CompactCmd() *cli.Command {
	return &cli.Command{
		Name:  "compact",
		Usage: "Archive stale and low-importance memories",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "days", Aliases: []string{"d"}, Value: 90,
				Usage: "Archive memories older than N days"},
			&cli.Float64Flag{Name: "threshold", Value: 0.4,
				Usage: "Archive active memories with importance below this value"},
			&cli.BoolFlag{Name: "dry-run", Usage: "Show count without making changes"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.Load()
			if err != nil {
				output.Fail(err.Error())
				return cli.Exit("", 1)
			}
			return runCompact(c.Context, cfg,
				c.Int("days"),
				c.Float64("threshold"),
				c.Bool("dry-run"),
			)
		},
	}
}

func runCompact(ctx context.Context, cfg *config.Config, days int, threshold float64, dryRun bool) error {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		output.Fail("Cannot connect to database: " + err.Error())
		return cli.Exit("", 1)
	}
	defer pool.Close()

	interval := fmt.Sprintf("%d days", days)
	countVal, err := db.FetchVal(ctx, pool, `
		SELECT count(*) FROM memory_events
		WHERE (metadata->>'status' = 'superseded' OR importance < $1)
		  AND created_at < now() - $2::interval
		  AND (metadata->>'status' IS NULL OR metadata->>'status' != 'stale')`,
		threshold, interval)
	count := int64(0)
	if err == nil && countVal != nil {
		if n, ok := countVal.(int64); ok {
			count = n
		}
	}

	if dryRun {
		output.Warn(fmt.Sprintf("Dry run: would archive %d memories.", count))
		return nil
	}

	if err := db.Exec(ctx, pool, `
		UPDATE memory_events
		SET metadata = jsonb_set(metadata, '{status}', '"stale"')
		WHERE (metadata->>'status' = 'superseded' OR importance < $1)
		  AND created_at < now() - $2::interval
		  AND (metadata->>'status' IS NULL OR metadata->>'status' != 'stale')`,
		threshold, interval); err != nil {
		output.Fail("Compact failed: " + err.Error())
		return cli.Exit("", 1)
	}
	output.OK(fmt.Sprintf("Archived %d memories (marked as stale).", count))
	return nil
}

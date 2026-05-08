package cmd

import (
	"context"
	"fmt"

	"github.com/abishekkumar/claude-memory/cli/internal/config"
	"github.com/abishekkumar/claude-memory/cli/internal/db"
	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func ResetCmd() *cli.Command {
	return &cli.Command{
		Name:  "reset",
		Usage: "Delete memories (all or scoped to a project)",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Skip confirmation"},
			&cli.StringFlag{Name: "project", Aliases: []string{"p"}, Usage: "Delete only this project's memories"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.Load()
			if err != nil {
				output.Fail(err.Error())
				return cli.Exit("", 1)
			}
			return runReset(c.Context, cfg, c.String("project"), c.Bool("yes"))
		},
	}
}

func runReset(ctx context.Context, cfg *config.Config, project string, yes bool) error {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		output.Fail("Cannot connect to database: " + err.Error())
		return cli.Exit("", 1)
	}
	defer pool.Close()

	var countVal any
	if project != "" {
		countVal, _ = db.FetchVal(ctx, pool,
			"SELECT count(*) FROM memory_events WHERE metadata->>'project' = $1", project)
	} else {
		countVal, _ = db.FetchVal(ctx, pool, "SELECT count(*) FROM memory_events")
	}
	count := fmt.Sprint(countVal)
	scope := "ALL workspaces"
	if project != "" {
		scope = "project " + project
	}

	output.Warn(fmt.Sprintf("This will permanently delete %s memories from %s.", count, scope))
	if !yes {
		fmt.Print("Continue? [y/N]: ")
		var ans string
		fmt.Scanln(&ans)
		if ans != "y" && ans != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if project != "" {
		db.Exec(ctx, pool, "DELETE FROM memory_events WHERE metadata->>'project' = $1", project)
	} else {
		db.Exec(ctx, pool, "TRUNCATE memory_events")
	}
	output.OK(fmt.Sprintf("Deleted %s memories from %s.", count, scope))
	return nil
}

package cmd

import (
	"context"
	"fmt"

	"github.com/abishekkumar/claude-memory/cli/internal/ai"
	"github.com/abishekkumar/claude-memory/cli/internal/config"
	"github.com/abishekkumar/claude-memory/cli/internal/db"
	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func SearchCmd() *cli.Command {
	return &cli.Command{
		Name:      "search",
		Usage:     "Semantic search across memories",
		ArgsUsage: "<query>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Aliases: []string{"t"}, Usage: "Filter by memory_type"},
			&cli.StringFlag{Name: "project", Aliases: []string{"p"}, Usage: "Filter by project path"},
			&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Value: 10, Usage: "Max results"},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return cli.Exit("Usage: memory search <query>", 1)
			}
			cfg, err := config.Load()
			if err != nil {
				output.Fail(err.Error())
				return cli.Exit("", 1)
			}
			return runSearch(c.Context, cfg,
				c.Args().First(),
				c.String("type"),
				c.String("project"),
				c.Int("limit"),
			)
		},
	}
}

func runSearch(ctx context.Context, cfg *config.Config, query, memType, project string, limit int) error {
	aiClient := ai.NewClient(cfg.OpenAIKey)
	emb, err := aiClient.Embed(ctx, query)
	if err != nil {
		output.Fail("Embedding failed: " + err.Error())
		return cli.Exit("", 1)
	}
	vec := db.VecString(emb)

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		output.Fail("Cannot connect to database: " + err.Error())
		return cli.Exit("", 1)
	}
	defer pool.Close()

	var typeArg, projectArg interface{}
	if memType != "" {
		typeArg = memType
	}
	if project != "" {
		projectArg = project
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := db.Fetch(ctx, pool, `
		SELECT
			m.memory_type,
			m.subject,
			m.content,
			m.scope,
			m.created_at,
			1 - (m.embedding <=> $1::vector) AS similarity
		FROM memory_events m
		WHERE ($2::text IS NULL OR m.memory_type = $2)
		  AND ($3::text IS NULL
		       OR m.metadata->>'project' = $3
		       OR m.metadata->>'workspace_path' = $3)
		  AND (m.metadata->>'status' IS NULL OR m.metadata->>'status' = 'active')
		ORDER BY m.embedding <=> $1::vector
		LIMIT $4`,
		vec, typeArg, projectArg, limit,
	)
	if err != nil {
		output.Fail("Search failed: " + err.Error())
		return cli.Exit("", 1)
	}

	if len(rows) == 0 {
		output.Warn("No results found.")
		return nil
	}

	output.Bold(fmt.Sprintf("\nResults for: %s\n", query))
	t := output.Table("Score", "Type", "Subject", "Content", "Scope", "Date")
	for _, r := range rows {
		score := fmt.Sprintf("%.2f", r["similarity"])
		content := fmt.Sprint(r["content"])
		if len(content) > 80 {
			content = content[:80] + "…"
		}
		date := fmt.Sprint(r["created_at"])
		if len(date) > 10 {
			date = date[:10]
		}
		subject := fmt.Sprint(r["subject"])
		if subject == "<nil>" {
			subject = "—"
		}
		t.AppendRow(table.Row{score, r["memory_type"], subject, content, r["scope"], date})
	}
	t.Render()
	fmt.Println()
	return nil
}

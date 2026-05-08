package cmd

import (
	"context"
	"fmt"

	"github.com/abishekkumar/claude-memory/cli/internal/ai"
	"github.com/abishekkumar/claude-memory/cli/internal/config"
	"github.com/abishekkumar/claude-memory/cli/internal/db"
	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func ReindexCmd() *cli.Command {
	return &cli.Command{
		Name:  "reindex",
		Usage: "Re-embed all memories (use after changing embedding model)",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "batch-size", Value: 50, Usage: "Embeddings per API call"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.Load()
			if err != nil {
				output.Fail(err.Error())
				return cli.Exit("", 1)
			}
			return runReindex(c.Context, cfg, c.Int("batch-size"))
		},
	}
}

func runReindex(ctx context.Context, cfg *config.Config, batchSize int) error {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		output.Fail("Cannot connect to database: " + err.Error())
		return cli.Exit("", 1)
	}
	defer pool.Close()

	rows, err := db.Fetch(ctx, pool,
		"SELECT id::text, content FROM memory_events WHERE content IS NOT NULL ORDER BY created_at")
	if err != nil {
		output.Fail("Fetch failed: " + err.Error())
		return cli.Exit("", 1)
	}

	total := len(rows)
	output.Info(fmt.Sprintf("Re-embedding %d memories in batches of %d...", total, batchSize))

	aiClient := ai.NewClient(cfg.OpenAIKey)

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := rows[i:end]
		texts := make([]string, len(batch))
		for j, r := range batch {
			texts[j] = fmt.Sprint(r["content"])
		}
		embeddings, err := aiClient.EmbedBatch(ctx, texts)
		if err != nil {
			output.Fail("Embedding batch failed: " + err.Error())
			return cli.Exit("", 1)
		}
		for j, emb := range embeddings {
			vec := db.VecString(emb)
			if err := db.Exec(ctx, pool,
				"UPDATE memory_events SET embedding = $1::vector WHERE id = $2::uuid",
				vec, fmt.Sprint(batch[j]["id"])); err != nil {
				output.Fail("Update failed: " + err.Error())
				return cli.Exit("", 1)
			}
		}
		fmt.Printf("\r  %d / %d", end, total)
	}
	fmt.Println()
	output.OK(fmt.Sprintf("Re-indexed %d memories.", total))
	return nil
}

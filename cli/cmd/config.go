package cmd

import (
	"fmt"
	"os"

	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func ConfigCmd() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Show current configuration",
		Action: func(c *cli.Context) error {
			dbURL := os.Getenv("DATABASE_URL")
			apiKey := os.Getenv("OPENAI_API_KEY")

			statusStr := func(ok bool) string {
				if ok {
					return "✓"
				}
				return "✗"
			}
			maskedKey := apiKey
			if len(apiKey) > 7 {
				maskedKey = apiKey[:7] + "…" + apiKey[len(apiKey)-4:]
			}

			output.Bold("\nClaude Memory — Configuration\n")
			t := output.Table("Variable", "Value", "Status")
			t.AppendRow(table.Row{"DATABASE_URL", dbURL, statusStr(dbURL != "")})
			t.AppendRow(table.Row{"OPENAI_API_KEY", maskedKey, statusStr(apiKey != "")})
			t.Render()
			fmt.Println()
			return nil
		},
	}
}

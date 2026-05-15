package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/abishekkumar/claude-memory/cli/internal/ai"
	"github.com/abishekkumar/claude-memory/cli/internal/config"
	"github.com/abishekkumar/claude-memory/cli/internal/db"
	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v2"
)

func DoctorCmd() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Check system health — postgres, pgvector, schema, MCP, embeddings, roundtrip",
		Action: func(c *cli.Context) error {
			cfg, err := config.Load()
			if err != nil {
				output.Fail(err.Error())
				return cli.Exit("", 1)
			}
			return runDoctor(c.Context, cfg)
		},
	}
}

type checkResult struct {
	label  string
	ok     bool
	detail string
}

func runDoctor(ctx context.Context, cfg *config.Config) error {
	output.Bold("\nClaude Memory — Doctor\n")

	pool, poolErr := db.Connect(ctx, cfg.DatabaseURL)

	checks := []checkResult{
		checkPostgres(ctx, cfg.DatabaseURL),
		checkPgvector(ctx, pool, poolErr),
		checkSchema(ctx, pool, poolErr),
		checkMCP(),
		checkEmbeddings(ctx, cfg.OpenAIKey),
		checkRoundtrip(ctx, pool, poolErr, cfg.OpenAIKey),
	}

	if pool != nil {
		pool.Close()
	}

	t := output.Table("Check", "Status", "Detail")
	allOK := true
	for _, r := range checks {
		status := "✓"
		if !r.ok {
			status = "✗"
			allOK = false
		}
		t.AppendRow(table.Row{r.label, status, r.detail})
	}
	t.Render()
	fmt.Println()

	if allOK {
		output.OK("All checks passed.")
	} else {
		output.Fail("Some checks failed. Fix the issues above and re-run memory doctor.")
		return cli.Exit("", 1)
	}
	return nil
}

func checkPostgres(ctx context.Context, connStr string) checkResult {
	pool, err := db.Connect(ctx, connStr)
	if err != nil {
		return checkResult{"PostgreSQL reachable", false, err.Error()}
	}
	pool.Close()
	return checkResult{"PostgreSQL reachable", true, "reachable"}
}

func checkPgvector(ctx context.Context, pool *pgxpool.Pool, poolErr error) checkResult {
	if poolErr != nil {
		return checkResult{"pgvector extension", false, "skipped (no DB connection)"}
	}
	rows, err := db.Fetch(ctx, pool,
		"SELECT extversion FROM pg_extension WHERE extname = 'vector'")
	if err != nil || len(rows) == 0 {
		return checkResult{"pgvector extension", false, "extension not installed — run: CREATE EXTENSION vector;"}
	}
	ver := fmt.Sprintf("v%v installed", rows[0]["extversion"])
	return checkResult{"pgvector extension", true, ver}
}

func checkSchema(ctx context.Context, pool *pgxpool.Pool, poolErr error) checkResult {
	if poolErr != nil {
		return checkResult{"Schema tables", false, "skipped (no DB connection)"}
	}
	rows, err := db.Fetch(ctx, pool,
		"SELECT tablename FROM pg_tables WHERE schemaname = 'public'")
	if err != nil {
		return checkResult{"Schema tables", false, err.Error()}
	}
	have := map[string]bool{}
	for _, r := range rows {
		have[fmt.Sprint(r["tablename"])] = true
	}
	need := []string{"memory_events", "conversation_sessions", "messages"}
	var missing []string
	for _, n := range need {
		if !have[n] {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return checkResult{"Schema tables", false, "missing: " + strings.Join(missing, ", ")}
	}
	return checkResult{"Schema tables", true, "all tables present"}
}

func checkMCP() checkResult {
	out, err := exec.Command("claude", "mcp", "list").Output()
	if err != nil {
		return checkResult{"MCP server", false, "claude CLI not found or failed"}
	}
	s := string(out)
	if strings.Contains(s, "memory") && !strings.Contains(s, "✗") {
		return checkResult{"MCP server", true, "listed by claude mcp list (plugin or user scope)"}
	}
	if strings.Contains(s, "memory") {
		return checkResult{"MCP server", false, "listed but not connected — check DATABASE_URL / OPENAI_API_KEY for the MCP process"}
	}
	return checkResult{"MCP server", false, "not listed — run ./install.sh (installs claude-memory plugin) or enable the plugin manually; avoid duplicate user-scope MCP unless intended."}
}

func checkEmbeddings(ctx context.Context, apiKey string) checkResult {
	client := ai.NewClient(apiKey)
	emb, err := client.Embed(ctx, "doctor check")
	if err != nil {
		return checkResult{"Embeddings API", false, err.Error()}
	}
	return checkResult{"Embeddings API", true,
		fmt.Sprintf("text-embedding-ada-002 (%dd)", len(emb))}
}

func checkRoundtrip(ctx context.Context, pool *pgxpool.Pool, poolErr error, apiKey string) checkResult {
	if poolErr != nil {
		return checkResult{"Write/read roundtrip", false, "skipped (no DB connection)"}
	}
	aiClient := ai.NewClient(apiKey)
	emb, err := aiClient.Embed(ctx, "roundtrip check")
	if err != nil {
		return checkResult{"Write/read roundtrip", false, "embedding failed: " + err.Error()}
	}
	vec := db.VecString(emb)
	rows, err := db.Fetch(ctx, pool, `
		INSERT INTO memory_events
			(memory_type, content, scope, embedding, metadata)
		VALUES ('observation', 'doctor:roundtrip', 'temporary', $1::vector,
			'{"status":"active","source":"doctor"}'::jsonb)
		RETURNING id::text`, vec)
	if err != nil || len(rows) == 0 {
		return checkResult{"Write/read roundtrip", false, "write failed: " + fmt.Sprint(err)}
	}
	id := fmt.Sprint(rows[0]["id"])
	if err := db.Exec(ctx, pool, "DELETE FROM memory_events WHERE id = $1::uuid", id); err != nil {
		return checkResult{"Write/read roundtrip", false, "cleanup failed: " + err.Error()}
	}
	return checkResult{"Write/read roundtrip", true, "write → read → delete OK"}
}

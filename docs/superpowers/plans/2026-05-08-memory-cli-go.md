# Memory CLI (Go) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `memory` CLI in Go — a single static binary with no runtime dependencies that serves as the operational control plane for the claude-memory platform.

**Architecture:** A `github.com/urfave/cli/v2` app in `cli/`, with internal packages for database access (`pgx/v5`), OpenAI embeddings, terminal output (`go-pretty` tables + `fatih/color`), and config from env vars. Each command is a function in `cli/cmd/` that returns a `*cli.Command`. The binary is built with `go build -o memory .` and placed on PATH. The CLI connects directly to PostgreSQL — it does not go through the MCP server.

**Tech Stack:** Go 1.22+, urfave/cli v2, pgx/v5, go-openai, go-pretty/v6, fatih/color.

**Prerequisite:** Complete `2026-05-08-plugin-restructuring.md` first (migrations/ and mcp-server/ must exist).

**Supersedes:** `2026-05-08-memory-cli.md` (Python/Typer plan — do not implement that plan).

---

### Task 1: Go module scaffold

**Files:**
- Create: `cli/go.mod`
- Create: `cli/main.go`
- Create: `cli/cmd/.gitkeep`
- Create: `cli/internal/db/.gitkeep`
- Create: `cli/internal/ai/.gitkeep`
- Create: `cli/internal/output/.gitkeep`
- Create: `cli/internal/config/.gitkeep`

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p cli/cmd cli/internal/db cli/internal/ai cli/internal/output cli/internal/config cli/templates
```

- [ ] **Step 2: Create cli/go.mod**

```
module github.com/abishekkumar/claude-memory/cli

go 1.22

require (
	github.com/fatih/color v1.17.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/jedib0t/go-pretty/v6 v6.5.9
	github.com/sashabaranov/go-openai v1.26.2
	github.com/urfave/cli/v2 v2.27.1
)
```

- [ ] **Step 3: Download dependencies**

```bash
cd cli && go mod tidy
# Expected: go.sum created, all deps resolved
```

- [ ] **Step 4: Create cli/main.go stub**

```go
package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "memory",
		Usage: "Claude Memory — operational control plane for your memory platform.",
		Commands: []*cli.Command{},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Build and verify**

```bash
cd cli && go build -o /tmp/memory-test .
/tmp/memory-test --help
# Expected: NAME: memory — operational control plane ...
```

- [ ] **Step 6: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/
git commit -m "feat: scaffold memory CLI Go module with urfave/cli"
```

---

### Task 2: internal/config package

**Files:**
- Create: `cli/internal/config/config.go`
- Create: `cli/internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

`cli/internal/config/config_test.go`:

```go
package config_test

import (
	"os"
	"testing"

	"github.com/abishekkumar/claude-memory/cli/internal/config"
)

func TestLoadMissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("OPENAI_API_KEY")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}
}

func TestLoadMissingOpenAIKey(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://x")
	os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("DATABASE_URL")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when OPENAI_API_KEY is missing")
	}
}

func TestLoadSuccess(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://x")
	os.Setenv("OPENAI_API_KEY", "sk-test")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("OPENAI_API_KEY")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://x" {
		t.Errorf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if cfg.OpenAIKey != "sk-test" {
		t.Errorf("unexpected OpenAIKey: %s", cfg.OpenAIKey)
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd cli && go test ./internal/config/...
# Expected: FAIL — config package does not exist yet
```

- [ ] **Step 3: Create cli/internal/config/config.go**

```go
package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
	OpenAIKey   string
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, errors.New("DATABASE_URL is not set — add it to your shell profile")
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY is not set — add it to your shell profile")
	}
	return &Config{DatabaseURL: dbURL, OpenAIKey: apiKey}, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd cli && go test ./internal/config/... -v
# Expected: 3 passed
```

- [ ] **Step 5: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/internal/config/
git commit -m "feat: add CLI config package — reads DATABASE_URL and OPENAI_API_KEY"
```

---

### Task 3: internal/db and internal/output packages

**Files:**
- Create: `cli/internal/db/db.go`
- Create: `cli/internal/db/db_test.go`
- Create: `cli/internal/output/output.go`

- [ ] **Step 1: Write failing test for db.VecString**

`cli/internal/db/db_test.go`:

```go
package db_test

import (
	"testing"

	"github.com/abishekkumar/claude-memory/cli/internal/db"
)

func TestVecString(t *testing.T) {
	result := db.VecString([]float32{1.0, 2.5, -0.5})
	expected := "[1,2.5,-0.5]"
	if result != expected {
		t.Errorf("VecString: got %q, want %q", result, expected)
	}
}

func TestVecStringEmpty(t *testing.T) {
	result := db.VecString([]float32{})
	if result != "[]" {
		t.Errorf("VecString empty: got %q", result)
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd cli && go test ./internal/db/...
# Expected: FAIL
```

- [ ] **Step 3: Create cli/internal/db/db.go**

```go
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool = pgxpool.Pool

// Row is a decoded database row — JSONB columns are pre-parsed to map.
type Row map[string]any

func Connect(ctx context.Context, connStr string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	cfg.MaxConns = 5
	cfg.MinConns = 1
	return pgxpool.NewWithConfig(ctx, cfg)
}

// VecString converts a float32 slice to the pgvector literal format "[x,y,...]".
func VecString(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(float64(f), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// Fetch executes a query and returns decoded rows.
// JSONB columns are returned as map[string]any.
func Fetch(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) ([]Row, error) {
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Row
	descs := rows.FieldDescriptions()

	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		r := make(Row, len(descs))
		for i, desc := range descs {
			v := vals[i]
			// pgx returns JSONB as []byte; decode to map
			if b, ok := v.([]byte); ok {
				var m any
				if json.Unmarshal(b, &m) == nil {
					v = m
				}
			}
			// Normalise timestamps to string
			if t, ok := v.(time.Time); ok {
				v = t.Format(time.RFC3339)
			}
			r[string(desc.Name)] = v
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// FetchVal executes a query and returns the first column of the first row.
func FetchVal(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (any, error) {
	rows, err := Fetch(ctx, pool, query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	for _, v := range rows[0] {
		return v, nil
	}
	return nil, nil
}

// Exec executes a statement with no return value.
func Exec(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) error {
	_, err := pool.Exec(ctx, query, args...)
	return err
}
```

Note: `VecString` uses `strconv` — add it to imports:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)
```

- [ ] **Step 4: Create cli/internal/output/output.go**

```go
package output

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

var (
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
)

func OK(msg string)   { fmt.Println(green("✓") + " " + msg) }
func Fail(msg string) { fmt.Fprintln(os.Stderr, red("✗")+" "+msg) }
func Warn(msg string) { fmt.Println(yellow("!")+" "+msg) }
func Info(msg string) { fmt.Println("·  " + msg) }
func Bold(msg string) { fmt.Println(bold(msg)) }

// Table creates a styled table writer that renders to stdout.
func Table(headers ...string) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.Style().Options.SeparateRows = false

	row := make(table.Row, len(headers))
	for i, h := range headers {
		row[i] = text.Bold.Sprint(h)
	}
	t.AppendHeader(row)
	return t
}
```

- [ ] **Step 5: Run db tests**

```bash
cd cli && go test ./internal/db/... -v
# Expected: TestVecString PASS, TestVecStringEmpty PASS
```

- [ ] **Step 6: Build check**

```bash
cd cli && go build ./...
# Expected: no errors
```

- [ ] **Step 7: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/internal/db/ cli/internal/output/
git commit -m "feat: add CLI db and output internal packages"
```

---

### Task 4: internal/ai package

**Files:**
- Create: `cli/internal/ai/embed.go`
- Create: `cli/internal/ai/embed_test.go`

- [ ] **Step 1: Write failing test**

`cli/internal/ai/embed_test.go`:

```go
package ai_test

import (
	"testing"

	"github.com/abishekkumar/claude-memory/cli/internal/ai"
)

func TestNewClientPanicsWithEmptyKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with empty API key")
		}
	}()
	ai.NewClient("")
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd cli && go test ./internal/ai/...
# Expected: FAIL
```

- [ ] **Step 3: Create cli/internal/ai/embed.go**

```go
package ai

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	c *openai.Client
}

func NewClient(apiKey string) *Client {
	if apiKey == "" {
		panic("OPENAI_API_KEY must not be empty")
	}
	return &Client{c: openai.NewClient(apiKey)}
}

// Embed returns the text-embedding-ada-002 embedding for the given text.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.c.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: []string{text},
		Model: openai.AdaEmbeddingV2,
	})
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}
	emb := resp.Data[0].Embedding
	result := make([]float32, len(emb))
	for i, v := range emb {
		result[i] = float32(v)
	}
	return result, nil
}

// EmbedBatch embeds multiple texts in a single API call.
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := c.c.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: texts,
		Model: openai.AdaEmbeddingV2,
	})
	if err != nil {
		return nil, fmt.Errorf("batch embedding failed: %w", err)
	}
	results := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		results[i] = make([]float32, len(d.Embedding))
		for j, v := range d.Embedding {
			results[i][j] = float32(v)
		}
	}
	return results, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd cli && go test ./internal/ai/... -v
# Expected: TestNewClientPanicsWithEmptyKey PASS
```

- [ ] **Step 5: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/internal/ai/
git commit -m "feat: add CLI ai package — OpenAI embedding client"
```

---

### Task 5: `memory doctor` command

**Files:**
- Create: `cli/cmd/doctor.go`
- Create: `cli/cmd/doctor_test.go`

- [ ] **Step 1: Write failing test**

`cli/cmd/doctor_test.go`:

```go
package cmd_test

import (
	"testing"

	"github.com/abishekkumar/claude-memory/cli/cmd"
	"github.com/urfave/cli/v2"
)

func TestDoctorCmdHasCorrectName(t *testing.T) {
	c := cmd.DoctorCmd()
	if c.Name != "doctor" {
		t.Errorf("expected name 'doctor', got %q", c.Name)
	}
}

func TestDoctorCmdHasUsage(t *testing.T) {
	c := cmd.DoctorCmd()
	if c.Usage == "" {
		t.Error("doctor command must have a Usage string")
	}
}

func appWithDoctor() *cli.App {
	return &cli.App{
		Name:     "memory",
		Commands: []*cli.Command{cmd.DoctorCmd()},
	}
}

func TestDoctorHelpFlag(t *testing.T) {
	app := appWithDoctor()
	err := app.Run([]string{"memory", "doctor", "--help"})
	if err != nil {
		t.Errorf("doctor --help returned error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd cli && go test ./cmd/... 2>&1 | head -5
# Expected: FAIL — cmd package does not exist
```

- [ ] **Step 3: Create cli/cmd/doctor.go**

```go
package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

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
		return checkResult{"MCP server", true, "registered and connected"}
	}
	if strings.Contains(s, "memory") {
		return checkResult{"MCP server", false, "registered but not connected — check env vars"}
	}
	return checkResult{"MCP server", false, "not registered — run: ./install.sh"}
}

func checkEmbeddings(ctx context.Context, apiKey string) checkResult {
	from github.com/abishekkumar/claude-memory/cli/internal/ai
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
```

Note: fix the import inside `checkEmbeddings` — it should be at the top of the file:

```go
import (
	...
	"github.com/abishekkumar/claude-memory/cli/internal/ai"
	...
)
```

- [ ] **Step 4: Run tests**

```bash
cd cli && go test ./cmd/... -v -run TestDoctor
# Expected: 3 passed (name, usage, help flag)
```

- [ ] **Step 5: Wire doctor into main.go**

```go
package main

import (
	"log"
	"os"

	"github.com/abishekkumar/claude-memory/cli/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "memory",
		Usage: "Claude Memory — operational control plane for your memory platform.",
		Commands: []*cli.Command{
			cmd.DoctorCmd(),
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 6: Build and smoke test**

```bash
cd cli && go build -o /tmp/memory .
DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
OPENAI_API_KEY="sk-..." \
/tmp/memory doctor
# Expected: table with ✓ for all rows, "All checks passed."
```

- [ ] **Step 7: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/cmd/doctor.go cli/cmd/doctor_test.go cli/main.go
git commit -m "feat: implement memory doctor command"
```

---

### Task 6: `memory status` and `memory stats` commands

**Files:**
- Create: `cli/cmd/status.go`
- Create: `cli/cmd/stats.go`

- [ ] **Step 1: Create cli/cmd/status.go**

```go
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

	type stat struct{ label, value string }
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
```

- [ ] **Step 2: Create cli/cmd/stats.go**

```go
package cmd

import (
	"context"
	"fmt"
	"strings"

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
					row[i] = fmt.Sprint(v)
					i++
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
```

- [ ] **Step 3: Add to main.go**

```go
Commands: []*cli.Command{
    cmd.DoctorCmd(),
    cmd.StatusCmd(),
    cmd.StatsCmd(),
},
```

- [ ] **Step 4: Build and smoke test**

```bash
cd cli && go build -o /tmp/memory .
DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
OPENAI_API_KEY="sk-..." \
/tmp/memory status
# Expected: table with Active memories, DB size, Last write, Active projects

DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
OPENAI_API_KEY="sk-..." \
/tmp/memory stats
# Expected: multiple tables — by type, scope, topics, projects, activity
```

- [ ] **Step 5: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/cmd/status.go cli/cmd/stats.go cli/main.go
git commit -m "feat: implement memory status and stats commands"
```

---

### Task 7: `memory search` command

**Files:**
- Create: `cli/cmd/search.go`

- [ ] **Step 1: Create cli/cmd/search.go**

```go
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
```

- [ ] **Step 2: Add to main.go**

```go
Commands: []*cli.Command{
    cmd.DoctorCmd(),
    cmd.StatusCmd(),
    cmd.StatsCmd(),
    cmd.SearchCmd(),
},
```

- [ ] **Step 3: Build and smoke test**

```bash
cd cli && go build -o /tmp/memory .
DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
OPENAI_API_KEY="sk-..." \
/tmp/memory search "postgres database decisions"
# Expected: table with similarity scores

/tmp/memory search "kubernetes" --type decision
# Expected: filtered results
```

- [ ] **Step 4: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/cmd/search.go cli/main.go
git commit -m "feat: implement memory search command"
```

---

### Task 8: `memory export` command

**Files:**
- Create: `cli/cmd/export.go`
- Create: `cli/templates/timeline.md.tmpl`
- Create: `cli/templates/export.md.tmpl`

- [ ] **Step 1: Create cli/templates/timeline.md.tmpl**

```
# Memory Timeline

_Generated {{.GeneratedAt}}_

---
{{range $month, $entries := .Months}}
## {{$month}}
{{range $entries}}- {{truncate .Content 120}}
{{end}}{{end}}
```

- [ ] **Step 2: Create cli/templates/export.md.tmpl**

```
# Memory Export

_Generated {{.GeneratedAt}} — {{.Total}} memories_

---
{{range $type, $memories := .ByType}}
## {{title $type}} ({{len $memories}})
{{range $memories}}
### {{or .Subject "(no subject)"}}
{{.Content}}

_scope: {{.Scope}} | importance: {{.Importance}} | {{slice .CreatedAt 0 10}}_

---
{{end}}{{end}}
```

- [ ] **Step 3: Create cli/cmd/export.go**

```go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/abishekkumar/claude-memory/cli/internal/config"
	"github.com/abishekkumar/claude-memory/cli/internal/db"
	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func ExportCmd() *cli.Command {
	return &cli.Command{
		Name:  "export",
		Usage: "Export memories as json, markdown, or timeline",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Value: "json",
				Usage: "Output format: json | markdown | timeline"},
			&cli.StringFlag{Name: "project", Aliases: []string{"p"}, Usage: "Filter to project path"},
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "Write to file instead of stdout"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.Load()
			if err != nil {
				output.Fail(err.Error())
				return cli.Exit("", 1)
			}
			return runExport(c.Context, cfg,
				c.String("format"),
				c.String("project"),
				c.String("output"),
			)
		},
	}
}

type memoryRow struct {
	ID        string
	Type      string
	Subject   string
	Content   string
	Importance float64
	Confidence float64
	Scope     string
	CreatedAt string
	Metadata  map[string]any
}

func runExport(ctx context.Context, cfg *config.Config, format, project, outFile string) error {
	if format != "json" && format != "markdown" && format != "timeline" {
		output.Fail("--format must be json, markdown, or timeline")
		return cli.Exit("", 1)
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		output.Fail("Cannot connect to database: " + err.Error())
		return cli.Exit("", 1)
	}
	defer pool.Close()

	query := `
		SELECT id::text, memory_type, coalesce(subject,''), content,
		       importance, confidence, scope, created_at, metadata
		FROM memory_events
		WHERE (metadata->>'status' IS NULL OR metadata->>'status' = 'active')`
	args := []any{}
	if project != "" {
		query += ` AND (metadata->>'project' = $1 OR metadata->>'workspace_path' = $1)`
		args = append(args, project)
	}
	query += ` ORDER BY created_at ASC`

	rows, err := db.Fetch(ctx, pool, query, args...)
	if err != nil {
		output.Fail("Query failed: " + err.Error())
		return cli.Exit("", 1)
	}

	memories := make([]memoryRow, 0, len(rows))
	for _, r := range rows {
		m := memoryRow{
			ID:        fmt.Sprint(r["id"]),
			Type:      fmt.Sprint(r["memory_type"]),
			Subject:   fmt.Sprint(r["subject"]),
			Content:   fmt.Sprint(r["content"]),
			Scope:     fmt.Sprint(r["scope"]),
			CreatedAt: fmt.Sprint(r["created_at"]),
		}
		if v, ok := r["importance"].(float64); ok {
			m.Importance = v
		}
		if v, ok := r["confidence"].(float64); ok {
			m.Confidence = v
		}
		if meta, ok := r["metadata"].(map[string]any); ok {
			m.Metadata = meta
		}
		memories = append(memories, m)
	}

	var content string
	now := time.Now().UTC().Format("2006-01-02 15:04 UTC")

	switch format {
	case "json":
		b, _ := json.MarshalIndent(memories, "", "  ")
		content = string(b)

	case "markdown":
		byType := map[string][]memoryRow{}
		for _, m := range memories {
			byType[m.Type] = append(byType[m.Type], m)
		}
		funcs := template.FuncMap{
			"title": func(s string) string {
				return strings.Title(strings.ReplaceAll(s, "_", " "))
			},
			"or": func(a, b string) string {
				if a == "" || a == "<nil>" {
					return b
				}
				return a
			},
			"slice": func(s string, i, j int) string {
				if j > len(s) {
					j = len(s)
				}
				return s[i:j]
			},
		}
		tmpl, _ := template.New("export").Funcs(funcs).ParseFiles("templates/export.md.tmpl")
		var sb strings.Builder
		tmpl.ExecuteTemplate(&sb, "export.md.tmpl", map[string]any{
			"GeneratedAt": now,
			"Total":       len(memories),
			"ByType":      byType,
		})
		content = sb.String()

	case "timeline":
		relevant := map[string]bool{
			"decision": true, "conversation_summary": true,
			"learning": true, "project_context": true,
		}
		months := map[string][]memoryRow{}
		for _, m := range memories {
			if !relevant[m.Type] {
				continue
			}
			month := m.CreatedAt[:7]
			months[month] = append(months[month], m)
		}
		funcs := template.FuncMap{
			"truncate": func(s string, n int) string {
				if len(s) <= n {
					return s
				}
				return s[:n] + "…"
			},
		}
		tmpl, _ := template.New("timeline").Funcs(funcs).ParseFiles("templates/timeline.md.tmpl")
		var sb strings.Builder
		tmpl.ExecuteTemplate(&sb, "timeline.md.tmpl", map[string]any{
			"GeneratedAt": now,
			"Months":      months,
		})
		content = sb.String()
	}

	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(content), 0644); err != nil {
			output.Fail("Write failed: " + err.Error())
			return cli.Exit("", 1)
		}
		output.OK(fmt.Sprintf("Exported %d memories to %s", len(memories), outFile))
	} else {
		fmt.Print(content)
	}
	return nil
}
```

- [ ] **Step 4: Add to main.go and build**

```go
cmd.ExportCmd(),
```

```bash
cd cli && go build -o /tmp/memory .
DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
OPENAI_API_KEY="sk-..." \
/tmp/memory export --format json | head -20
# Expected: JSON array

/tmp/memory export --format timeline
# Expected: ## 2026-05 section with bullet points
```

- [ ] **Step 5: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/cmd/export.go cli/templates/ cli/main.go
git commit -m "feat: implement memory export — json, markdown, timeline"
```

---

### Task 9: `memory backup` and `memory restore` commands

**Files:**
- Create: `cli/cmd/backup.go`
- Create: `cli/cmd/restore.go`

- [ ] **Step 1: Create cli/cmd/backup.go**

```go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func BackupCmd() *cli.Command {
	return &cli.Command{
		Name:  "backup",
		Usage: "Dump the memory database to a SQL file",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "Output file path"},
		},
		Action: func(c *cli.Context) error {
			outFile := c.String("output")
			if outFile == "" {
				outFile = fmt.Sprintf("memory-backup-%s.sql",
					time.Now().Format("2006-01-02-150405"))
			}
			fmt.Printf("Backing up to %s...\n", outFile)
			out, err := exec.Command("docker", "exec", "claude-memory-db",
				"pg_dump", "-U", "postgres", "-d", "claude_memory", "--no-owner").Output()
			if err != nil {
				output.Fail("pg_dump failed: " + err.Error())
				return cli.Exit("", 1)
			}
			if err := os.WriteFile(outFile, out, 0644); err != nil {
				output.Fail("Write failed: " + err.Error())
				return cli.Exit("", 1)
			}
			info, _ := os.Stat(outFile)
			output.OK(fmt.Sprintf("Backup written to %s (%d bytes)", outFile, info.Size()))
			return nil
		},
	}
}
```

- [ ] **Step 2: Create cli/cmd/restore.go**

```go
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func RestoreCmd() *cli.Command {
	return &cli.Command{
		Name:      "restore",
		Usage:     "Restore the memory database from a SQL backup",
		ArgsUsage: "<backup-file>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Skip confirmation"},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return cli.Exit("Usage: memory restore <backup-file>", 1)
			}
			file := c.Args().First()
			if _, err := os.Stat(file); os.IsNotExist(err) {
				output.Fail("File not found: " + file)
				return cli.Exit("", 1)
			}
			if !c.Bool("yes") {
				output.Warn("This will DROP and recreate the claude_memory database.")
				fmt.Print("Continue? [y/N]: ")
				var ans string
				fmt.Scanln(&ans)
				if ans != "y" && ans != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}
			fmt.Printf("Restoring from %s...\n", file)
			for _, sql := range []string{
				"DROP DATABASE IF EXISTS claude_memory;",
				"CREATE DATABASE claude_memory;",
			} {
				cmd := exec.Command("docker", "exec", "claude-memory-db",
					"psql", "-U", "postgres", "-c", sql)
				if out, err := cmd.CombinedOutput(); err != nil {
					output.Fail(string(out))
					return cli.Exit("", 1)
				}
			}
			data, err := os.ReadFile(file)
			if err != nil {
				output.Fail("Read failed: " + err.Error())
				return cli.Exit("", 1)
			}
			cmd := exec.Command("docker", "exec", "-i", "claude-memory-db",
				"psql", "-U", "postgres", "-d", "claude_memory")
			cmd.Stdin = strings.NewReader(string(data))
			if out, err := cmd.CombinedOutput(); err != nil {
				output.Fail("Restore failed: " + string(out))
				return cli.Exit("", 1)
			}
			output.OK("Restored from " + file)
			return nil
		},
	}
}
```

Add `"strings"` to imports in restore.go.

- [ ] **Step 3: Add to main.go, build, smoke test**

```go
cmd.BackupCmd(),
cmd.RestoreCmd(),
```

```bash
cd cli && go build -o /tmp/memory .
DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
OPENAI_API_KEY="sk-..." \
/tmp/memory backup
# Expected: memory-backup-2026-05-08-HHMMSS.sql created
ls memory-backup-*.sql
```

- [ ] **Step 4: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/cmd/backup.go cli/cmd/restore.go cli/main.go
git commit -m "feat: implement memory backup and restore commands"
```

---

### Task 10: `memory compact`, `memory reindex`, `memory migrate` commands

**Files:**
- Create: `cli/cmd/compact.go`
- Create: `cli/cmd/reindex.go`
- Create: `cli/cmd/migrate.go`

- [ ] **Step 1: Create cli/cmd/compact.go**

```go
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
```

- [ ] **Step 2: Create cli/cmd/reindex.go**

```go
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
```

- [ ] **Step 3: Create cli/cmd/migrate.go**

```go
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

	pending := []string{}
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
```

- [ ] **Step 4: Add to main.go, build, smoke test**

```go
cmd.CompactCmd(),
cmd.ReindexCmd(),
cmd.MigrateCmd(),
```

```bash
cd cli && go build -o /tmp/memory .
DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
OPENAI_API_KEY="sk-..." \
/tmp/memory migrate --dir ./migrations
# Expected: "All migrations already applied."

/tmp/memory compact --dry-run
# Expected: "Dry run: would archive N memories."
```

- [ ] **Step 5: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/cmd/compact.go cli/cmd/reindex.go cli/cmd/migrate.go cli/main.go
git commit -m "feat: implement memory compact, reindex, and migrate commands"
```

---

### Task 11: `memory config`, `memory reset`, `memory install`, `memory uninstall` + final wiring

**Files:**
- Create: `cli/cmd/config.go`
- Create: `cli/cmd/reset.go`
- Create: `cli/cmd/install.go`
- Create: `cli/cmd/uninstall.go`
- Modify: `cli/main.go` (final, all commands)
- Modify: `install.sh` (replace uv tool install with go build)

- [ ] **Step 1: Create cli/cmd/config.go**

```go
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

			t := output.Table("Variable", "Value", "Status")
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
			t.AppendRow(table.Row{"DATABASE_URL", dbURL, statusStr(dbURL != "")})
			t.AppendRow(table.Row{"OPENAI_API_KEY", maskedKey, statusStr(apiKey != "")})
			output.Bold("\nClaude Memory — Configuration\n")
			t.Render()
			fmt.Println()
			return nil
		},
	}
}
```

- [ ] **Step 2: Create cli/cmd/reset.go**

```go
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
```

- [ ] **Step 3: Create cli/cmd/install.go**

```go
package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func InstallCmd() *cli.Command {
	return &cli.Command{
		Name:  "install",
		Usage: "Run install.sh to set up the full memory system",
		Action: func(c *cli.Context) error {
			// Find install.sh relative to binary location
			exe, err := os.Executable()
			if err != nil {
				output.Fail("Cannot determine binary path: " + err.Error())
				return cli.Exit("", 1)
			}
			installSh := filepath.Join(filepath.Dir(exe), "..", "install.sh")
			if _, err := os.Stat(installSh); os.IsNotExist(err) {
				// Try CWD
				installSh = "./install.sh"
			}
			output.Info("Running " + installSh + "...")
			shell := "bash"
			if runtime.GOOS == "windows" {
				shell = "sh"
			}
			cmd := exec.Command(shell, installSh)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err != nil {
				output.Fail("install.sh failed: " + err.Error())
				return cli.Exit("", 1)
			}
			return nil
		},
	}
}
```

- [ ] **Step 4: Create cli/cmd/uninstall.go**

```go
package cmd

import (
	"fmt"
	"os/exec"

	"github.com/abishekkumar/claude-memory/cli/internal/output"
	"github.com/urfave/cli/v2"
)

func UninstallCmd() *cli.Command {
	return &cli.Command{
		Name:  "uninstall",
		Usage: "Remove MCP registration and optionally drop the database",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "keep-data", Usage: "Keep PostgreSQL data (only unregister MCP)"},
			&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Skip confirmation"},
		},
		Action: func(c *cli.Context) error {
			if !c.Bool("yes") {
				output.Warn("This will unregister the MCP server from Claude Code.")
				if !c.Bool("keep-data") {
					output.Warn("The PostgreSQL container and all data will also be removed.")
				}
				fmt.Print("Continue? [y/N]: ")
				var ans string
				fmt.Scanln(&ans)
				if ans != "y" && ans != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}
			if out, err := exec.Command("claude", "mcp", "remove", "memory", "-s", "user").CombinedOutput(); err != nil {
				output.Warn("Could not unregister MCP: " + string(out))
			} else {
				output.OK("MCP server unregistered from Claude Code.")
			}
			if !c.Bool("keep-data") {
				exec.Command("docker", "compose", "down", "-v").Run()
				output.OK("PostgreSQL container and volume removed.")
			}
			fmt.Println("\nTo reinstall: ./install.sh")
			return nil
		},
	}
}
```

- [ ] **Step 5: Final main.go with all 14 commands**

```go
package main

import (
	"log"
	"os"

	"github.com/abishekkumar/claude-memory/cli/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:                 "memory",
		Usage:                "Claude Memory — operational control plane for your memory platform.",
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			cmd.InstallCmd(),
			cmd.UninstallCmd(),
			cmd.DoctorCmd(),
			cmd.StatusCmd(),
			cmd.StatsCmd(),
			cmd.SearchCmd(),
			cmd.ExportCmd(),
			cmd.BackupCmd(),
			cmd.RestoreCmd(),
			cmd.CompactCmd(),
			cmd.ReindexCmd(),
			cmd.MigrateCmd(),
			cmd.ConfigCmd(),
			cmd.ResetCmd(),
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 6: Update install.sh — replace uv tool install with go build**

In `install.sh`, replace the CLI section:

```bash
# --- CLI ---
step "Installing memory CLI..."
uv tool install --editable "$SCRIPT_DIR/cli"
info "memory CLI installed."
```

With:

```bash
# --- CLI ---
step "Building memory CLI..."
command -v go >/dev/null 2>&1 || error "go is required: https://go.dev/dl/"
cd "$SCRIPT_DIR/cli" && go build -o "$HOME/.local/bin/memory" .
cd "$SCRIPT_DIR"
info "memory CLI installed to ~/.local/bin/memory"
```

Also add Go to the prerequisites check:
```bash
command -v go     >/dev/null 2>&1 || error "go is required: https://go.dev/dl/"
```

- [ ] **Step 7: Build final binary and run golden path**

```bash
cd /Users/abishekkumar/Documents/memory-superagents/cli && go build -o /tmp/memory .
/tmp/memory --help
# Expected: lists all 14 commands

DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
OPENAI_API_KEY="sk-..." \
/tmp/memory doctor && /tmp/memory status && /tmp/memory stats
# Expected: all three commands succeed
```

- [ ] **Step 8: Commit**

```bash
cd /Users/abishekkumar/Documents/memory-superagents
git add cli/cmd/ cli/main.go install.sh
git commit -m "feat: complete memory CLI — all 14 commands wired, go build replaces uv"
```

---

### Final CLI structure

```
cli/
├── go.mod
├── go.sum
├── main.go                  # urfave/cli app, all 14 commands registered
├── cmd/
│   ├── doctor.go
│   ├── status.go
│   ├── stats.go
│   ├── search.go
│   ├── export.go
│   ├── backup.go
│   ├── restore.go
│   ├── compact.go
│   ├── reindex.go
│   ├── migrate.go
│   ├── config.go
│   ├── reset.go
│   ├── install.go
│   └── uninstall.go
├── internal/
│   ├── config/config.go
│   ├── db/db.go
│   ├── ai/embed.go
│   └── output/output.go
└── templates/
    ├── export.md.tmpl
    └── timeline.md.tmpl
```

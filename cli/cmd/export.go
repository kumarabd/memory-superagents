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
	ID         string
	Type       string
	Subject    string
	Content    string
	Importance float64
	Confidence float64
	Scope      string
	CreatedAt  string
	Metadata   map[string]any
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
				s = strings.ReplaceAll(s, "_", " ")
				if len(s) == 0 {
					return s
				}
				return strings.ToUpper(s[:1]) + s[1:]
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
		tmplPath := findTemplate("export.md.tmpl")
		tmpl, err := template.New("export.md.tmpl").Funcs(funcs).ParseFiles(tmplPath)
		if err != nil {
			output.Fail("Template parse error: " + err.Error())
			return cli.Exit("", 1)
		}
		var sb strings.Builder
		if err := tmpl.ExecuteTemplate(&sb, "export.md.tmpl", map[string]any{
			"GeneratedAt": now,
			"Total":       len(memories),
			"ByType":      byType,
		}); err != nil {
			output.Fail("Template execute error: " + err.Error())
			return cli.Exit("", 1)
		}
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
			if len(m.CreatedAt) < 7 {
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
		tmplPath := findTemplate("timeline.md.tmpl")
		tmpl, err := template.New("timeline.md.tmpl").Funcs(funcs).ParseFiles(tmplPath)
		if err != nil {
			output.Fail("Template parse error: " + err.Error())
			return cli.Exit("", 1)
		}
		var sb strings.Builder
		if err := tmpl.ExecuteTemplate(&sb, "timeline.md.tmpl", map[string]any{
			"GeneratedAt": now,
			"Months":      months,
		}); err != nil {
			output.Fail("Template execute error: " + err.Error())
			return cli.Exit("", 1)
		}
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

// findTemplate resolves the template path relative to CWD or the binary location.
func findTemplate(name string) string {
	candidates := []string{
		"templates/" + name,
		"cli/templates/" + name,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "templates/" + name
}

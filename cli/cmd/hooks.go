package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/google/uuid"
	openai "github.com/sashabaranov/go-openai"
	"github.com/abishekkumar/claude-memory/cli/internal/db"
	"github.com/urfave/cli/v2"
)

// defaultNotebookPayload matches mcp-server/common/db.py default_notebook_payload().
func defaultNotebookPayload() map[string]any {
	return map[string]any{
		"term_cache":       map[string]any{},
		"concept_mapping":  map[string]any{},
		"preferences":      map[string]any{"row_cap": 100},
		"hypotheses":       []any{},
		"experiments":      []any{},
		"findings":         []any{},
		"semantic_links":   []any{},
		"open_questions":   []any{},
		"artifacts":        []any{},
	}
}

// notebookTouchEnsure mirrors MCP notebook.load first-touch: insert default row if missing, then return version.
func notebookTouchEnsure(ctx context.Context, pool *db.Pool, workspaceKey string) (version int64, err error) {
	if strings.TrimSpace(workspaceKey) == "" {
		return 0, fmt.Errorf("empty workspace_key")
	}
	payload, err := json.Marshal(defaultNotebookPayload())
	if err != nil {
		return 0, err
	}
	err = pool.QueryRow(ctx, `
		SELECT version FROM agentlab_notebook WHERE workspace_key = $1
	`, workspaceKey).Scan(&version)
	if err == nil {
		return version, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	// Row missing — materialize default (same first-touch as MCP notebook.load).
	_, execErr := pool.Exec(ctx, `
		INSERT INTO agentlab_notebook (workspace_key, payload, version, updated_at)
		VALUES ($1, $2::jsonb, 1, now())
		ON CONFLICT (workspace_key) DO NOTHING
	`, workspaceKey, payload)
	if execErr != nil {
		return 0, execErr
	}
	err = pool.QueryRow(ctx, `
		SELECT version FROM agentlab_notebook WHERE workspace_key = $1
	`, workspaceKey).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// HookInput is a partial schema of Claude Code hook stdin JSON.
// We only rely on fields we need for deterministic session wiring.
type HookInput struct {
	SessionID      string `json:"session_id"`
	Cwd            string `json:"cwd"`
	Source         string `json:"source"`
	Model          string `json:"model"`
	AgentType      string `json:"agent_type"`
	EventName      string `json:"hook_event_name"`
	TranscriptPath string `json:"transcript_path"`
	Reason         string `json:"reason"`
}

func HooksCmd() *cli.Command {
	return &cli.Command{
		Name:  "hook",
		Usage: "Hook helpers for Claude Code (invoked by hooks.json)",
		Subcommands: []*cli.Command{
			HookSessionStartCmd(),
			HookSessionEndCmd(),
		},
	}
}

func HookSessionStartCmd() *cli.Command {
	return &cli.Command{
		Name:  "session-start",
		Usage: "Handle SessionStart hook input: CLAUDE_ENV_FILE exports, AgentLab notebook row (notebook.load materialize), additionalContext JSON",
		Action: func(c *cli.Context) error {
			in, err := readHookInput(os.Stdin)
			if err != nil {
				// Still emit hook JSON so the hook doesn't crash the session.
				return printHookAdditionalContext("SessionStart", "Claude Memory is enabled for this session.")
			}

			sid := strings.TrimSpace(in.SessionID)
			if sid == "" {
				sid = uuid.NewString()
			} else {
				// Validate UUID shape; if invalid, replace with generated UUID.
				if _, err := uuid.Parse(sid); err != nil {
					sid = uuid.NewString()
				}
			}

			cwd := strings.TrimSpace(in.Cwd)
			if cwd == "" {
				cwd = strings.TrimSpace(os.Getenv("CLAUDE_PROJECT_DIR"))
			}

			// Persist environment variables for subsequent commands in this Claude session.
			// CLAUDE_ENV_FILE is provided by Claude Code for SessionStart hooks.
			if envFile := os.Getenv("CLAUDE_ENV_FILE"); envFile != "" {
				if err := appendExports(envFile, map[string]string{
					"CLAUDE_CODE_SESSION_ID":     sid,
					"CLAUDE_CODE_SESSION_CWD":    cwd,
					"CLAUDE_CODE_SESSION_SOURCE": strings.TrimSpace(in.Source),
					"CLAUDE_CODE_MODEL":          strings.TrimSpace(in.Model),
					"CLAUDE_CODE_AGENT_TYPE":     strings.TrimSpace(in.AgentType),
				}); err != nil {
					// Ignore; hook should not block session.
				}
			}

			msg := "Claude Memory is enabled for this session. A Claude session id is available to tools via the CLAUDE_CODE_SESSION_ID environment variable."
			if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" && cwd != "" {
				nctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer cancel()
				pool, err := db.Connect(nctx, dbURL)
				if err != nil {
					fmt.Fprintf(os.Stderr, "memory hook session-start: notebook touch: connect: %v\n", err)
				} else {
					defer pool.Close()
					ver, err := notebookTouchEnsure(nctx, pool, cwd)
					if err != nil {
						fmt.Fprintf(os.Stderr, "memory hook session-start: notebook touch (same as MCP notebook.load materialize): %v — run `memory migrate` if agentlab_notebook is missing\n", err)
					} else {
						msg += fmt.Sprintf(" AgentLab notebook row is ready for this workspace (version %d); MCP notebook.load will return the same row.", ver)
					}
				}
			}

			return printHookAdditionalContext("SessionStart", msg)
		},
	}
}

func HookSessionEndCmd() *cli.Command {
	return &cli.Command{
		Name:  "session-end",
		Usage: "Handle SessionEnd hook input: summarize session and update ended_at + summary in DB",
		Action: func(c *cli.Context) error {
			in, err := readHookInput(os.Stdin)
			if err != nil {
				return nil // don't block session end
			}

			sid := strings.TrimSpace(in.SessionID)
			if sid == "" {
				return nil
			}
			if _, err := uuid.Parse(sid); err != nil {
				return nil
			}

			dbURL := os.Getenv("DATABASE_URL")
			if dbURL == "" {
				return nil
			}

			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			summary := summarizeFallback(in)
			if key := os.Getenv("OPENAI_API_KEY"); key != "" && strings.TrimSpace(in.TranscriptPath) != "" {
				if s, err := summarizeWithLLM(ctx, key, in.TranscriptPath); err == nil && strings.TrimSpace(s) != "" {
					summary = s
				}
			}

			pool, err := db.Connect(ctx, dbURL)
			if err != nil {
				return nil
			}
			defer pool.Close()

			meta := map[string]any{
				"reason":          strings.TrimSpace(in.Reason),
				"transcript_path": strings.TrimSpace(in.TranscriptPath),
				"cwd":             strings.TrimSpace(in.Cwd),
				"source":          strings.TrimSpace(in.Source),
				"model":           strings.TrimSpace(in.Model),
				"agent_type":      strings.TrimSpace(in.AgentType),
				"ended_from":      "hook",
				"hook":            "SessionEnd",
			}
			metaJSON, _ := json.Marshal(meta)

			_ = db.Exec(ctx, pool, `
				INSERT INTO conversation_sessions (id, workspace_path, agent_name, metadata)
				VALUES ($1::uuid, $2, 'claude-code', $3::jsonb)
				ON CONFLICT (id) DO NOTHING
			`, sid, strings.TrimSpace(in.Cwd), string(metaJSON))

			_ = db.Exec(ctx, pool, `
				UPDATE conversation_sessions
				SET ended_at = now(),
				    summary = COALESCE(NULLIF($2, ''), summary),
				    metadata = COALESCE(metadata, '{}'::jsonb) || $3::jsonb
				WHERE id = $1::uuid
			`, sid, strings.TrimSpace(summary), string(metaJSON))

			return nil
		},
	}
}

func readHookInput(r io.Reader) (*HookInput, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(b))
	if raw == "" {
		return &HookInput{}, nil
	}
	var in HookInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return nil, err
	}
	return &in, nil
}

func appendExports(path string, kv map[string]string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for k, v := range kv {
		if strings.TrimSpace(v) == "" {
			continue
		}
		// Avoid newlines in exports; keep it single-line.
		v = strings.ReplaceAll(v, "\n", " ")
		v = strings.ReplaceAll(v, "\r", " ")
		if _, err := fmt.Fprintf(w, "export %s=%q\n", k, v); err != nil {
			return err
		}
	}
	return w.Flush()
}

func printHookAdditionalContext(eventName, ctx string) error {
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     eventName,
			"additionalContext": ctx,
		},
	}
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(out)
}

func summarizeFallback(in *HookInput) string {
	parts := []string{"Session ended."}
	if strings.TrimSpace(in.Reason) != "" {
		parts = append(parts, "Reason: "+strings.TrimSpace(in.Reason)+".")
	}
	if strings.TrimSpace(in.Cwd) != "" {
		parts = append(parts, "Workspace: "+strings.TrimSpace(in.Cwd)+".")
	}
	return strings.Join(parts, " ")
}

func summarizeWithLLM(ctx context.Context, apiKey, transcriptPath string) (string, error) {
	raw, err := os.ReadFile(transcriptPath)
	if err != nil {
		return "", err
	}
	text := extractTranscriptText(string(raw), 20000)

	client := openai.NewClient(apiKey)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: "Summarize this Claude Code session concisely for a database summary field. " +
					"Return plain text (no markdown). Focus on: what was attempted, key decisions, problems, solutions, and next steps (if any).",
			},
			{Role: openai.ChatMessageRoleUser, Content: text},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func extractTranscriptText(jsonl string, maxChars int) string {
	var b strings.Builder
	sc := bufio.NewScanner(strings.NewReader(jsonl))
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 2*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(line), &obj) != nil {
			continue
		}
		role, _ := obj["role"].(string)
		content, _ := obj["content"].(string)
		if role == "" || content == "" {
			continue
		}
		fmt.Fprintf(&b, "%s: %s\n", role, content)
		if b.Len() >= maxChars {
			break
		}
	}
	s := b.String()
	if len(s) > maxChars {
		s = s[:maxChars]
	}
	return s
}


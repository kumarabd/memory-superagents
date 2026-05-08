package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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

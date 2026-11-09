// Package migrate applies the SQL files in migrations/ in name order, each in
// its own transaction, recording each in schema_migrations (ADR 0003).
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
)

// Apply runs every migration in fsys that schema_migrations doesn't record yet
// and returns the names it applied, in order.
func Apply(ctx context.Context, conn *pgx.Conn, fsys fs.FS) ([]string, error) {
	_, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`)
	if err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var applied []string
	for _, name := range names {
		done, err := applyOne(ctx, conn, fsys, name)
		if err != nil {
			return applied, err
		}
		if done {
			applied = append(applied, name)
		}
	}
	return applied, nil
}

// applyOne applies name unless schema_migrations already records it.
func applyOne(ctx context.Context, conn *pgx.Conn, fsys fs.FS, name string) (bool, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op
	// Serialise concurrent runners (two instances starting at once).
	if _, err := tx.Exec(ctx, `LOCK TABLE schema_migrations IN EXCLUSIVE MODE`); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1) ON CONFLICT DO NOTHING`, name)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, tx.Rollback(ctx) // already applied
	}
	sql, err := fs.ReadFile(fsys, name)
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		return false, fmt.Errorf("migration %s: %w", name, err)
	}
	return true, tx.Commit(ctx)
}

// Package testdb gives each acceptance test its own freshly migrated database
// (ADR 0005): real Postgres, every migration applied from empty, dropped when
// the test ends, so tests run in parallel and leave nothing behind.
//
// It connects as the migrate role through TEST_DATABASE_URL and hands the test
// an app-role URL built from TEST_APP_DATABASE_URL. Both default to the values
// scripts/dev-db.sh creates, so `go test ./...` works after that script with
// no further setup; CI sets the same values.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/rydncn/workspots/internal/migrate"
	"github.com/rydncn/workspots/migrations"
)

const (
	defaultAdminURL = "postgres://workspots_migrate:dev-migrate@localhost:5432/workspots_test"
	defaultAppURL   = "postgres://workspots_app:dev-app@localhost:5432/workspots_test"
)

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// New creates a migrated database for t and returns the migrate-role and
// app-role URLs for it. The database is dropped when t ends.
func New(t testing.TB) (adminURL, appURL string) {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, env("TEST_DATABASE_URL", defaultAdminURL))
	if err != nil {
		t.Fatalf("testdb: connect (is scripts/dev-db.sh running?): %v", err)
	}
	t.Cleanup(func() { admin.Close(ctx) })

	var suffix [6]byte
	_, _ = rand.Read(suffix[:])
	name := "test_" + hex.EncodeToString(suffix[:])
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("testdb: create database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("testdb: drop database %s: %v", name, err)
		}
	})

	adminURL = withDatabase(t, env("TEST_DATABASE_URL", defaultAdminURL), name)
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("testdb: connect to %s: %v", name, err)
	}
	defer conn.Close(ctx)
	if _, err := migrate.Apply(ctx, conn, migrations.FS); err != nil {
		t.Fatalf("testdb: migrate: %v", err)
	}
	return adminURL, withDatabase(t, env("TEST_APP_DATABASE_URL", defaultAppURL), name)
}

func withDatabase(t testing.TB, rawURL, name string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("testdb: bad URL: %v", err)
	}
	u.Path = "/" + name
	return fmt.Sprint(u)
}

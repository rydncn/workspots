package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/rydncn/workspots/internal/testdb"
)

// AC27: the process, not only config.Load, refuses to start in production
// with a development value, naming the variable.
func TestRunRefusesDevelopmentValuesInProduction(t *testing.T) {
	env := map[string]string{
		"WORKSPOTS_ENV": "production",
		"BASE_URL":      "https://workspots.example",
		"DATABASE_URL":  "postgres://workspots_app:dev-app@localhost:5432/workspots_dev",
	}
	err := run(nil, func(k string) string { return env[k] })
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || strings.Contains(err.Error(), "dev-app") {
		t.Fatalf("got %v", err)
	}
}

// grant-role makes the first admin from the command line (slice 5), and
// refuses an unknown address or role.
func TestGrantRole(t *testing.T) {
	t.Parallel()
	adminURL, _ := testdb.New(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `INSERT INTO users (email, password_hash) VALUES ('first@example.test', 'x')`); err != nil {
		t.Fatal(err)
	}
	env := func(k string) string { return map[string]string{"MIGRATE_DATABASE_URL": adminURL}[k] }
	if err := run([]string{"grant-role", "First@example.test", "admin"}, env); err != nil {
		t.Fatal(err)
	}
	var role string
	_ = conn.QueryRow(ctx, `SELECT role FROM users WHERE email = 'first@example.test'`).Scan(&role)
	if role != "admin" {
		t.Errorf("role %q", role)
	}
	if err := run([]string{"grant-role", "nobody@example.test", "admin"}, env); err == nil || !strings.Contains(err.Error(), "no account") {
		t.Errorf("unknown address: %v", err)
	}
	if err := run([]string{"grant-role", "first@example.test", "owner"}, env); err == nil || !strings.Contains(err.Error(), "role") {
		t.Errorf("unknown role: %v", err)
	}
	if err := run([]string{"grant-role"}, env); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("no arguments: %v", err)
	}
}

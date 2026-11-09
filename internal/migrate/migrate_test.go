package migrate_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/rydncn/workspots/internal/migrate"
	"github.com/rydncn/workspots/internal/testdb"
	"github.com/rydncn/workspots/migrations"
)

// AC30: the app role owns nothing and has only what its queries need, checked
// in the catalogue after every migration has run on an empty database (rule 2).
func TestAC30AppRolePrivileges(t *testing.T) {
	t.Parallel()
	adminURL, _ := testdb.New(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	count := func(name, sql string, want int) {
		t.Helper()
		var got int
		if err := conn.QueryRow(ctx, sql).Scan(&got); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: got %d, want %d", name, got, want)
		}
	}
	count("objects owned by workspots_app",
		`SELECT count(*) FROM pg_class c JOIN pg_roles r ON r.oid = c.relowner WHERE r.rolname = 'workspots_app'`, 0)
	count("PUBLIC privileges on schema public",
		`SELECT count(*) FROM pg_namespace n, aclexplode(n.nspacl) a WHERE n.nspname = 'public' AND a.grantee = 0`, 0)
	count("CREATE on schema public for workspots_app",
		`SELECT count(*) FROM (SELECT 1 WHERE has_schema_privilege('workspots_app', 'public', 'CREATE')) s`, 0)
	count("USAGE on schema public for workspots_app",
		`SELECT count(*) FROM (SELECT 1 WHERE has_schema_privilege('workspots_app', 'public', 'USAGE')) s`, 1)
	count("table privileges beyond SELECT, INSERT, UPDATE, DELETE",
		`SELECT count(*) FROM information_schema.role_table_grants
		 WHERE grantee = 'workspots_app' AND privilege_type NOT IN ('SELECT', 'INSERT', 'UPDATE', 'DELETE')`, 0)
	count("write privileges on views for workspots_app",
		`SELECT count(*) FROM information_schema.role_table_grants g JOIN information_schema.views v
		   ON v.table_schema = g.table_schema AND v.table_name = g.table_name
		 WHERE g.grantee = 'workspots_app' AND g.privilege_type <> 'SELECT'`, 0)
	count("views running with owner privileges",
		`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = 'public' AND c.relkind = 'v'
		   AND NOT coalesce((SELECT option_value::bool FROM pg_options_to_table(c.reloptions) WHERE option_name = 'security_invoker'), false)`, 0)
	count("privileges on schema_migrations",
		`SELECT count(*) FROM information_schema.role_table_grants
		 WHERE grantee = 'workspots_app' AND table_name = 'schema_migrations'`, 0)
	count("application tables without SELECT for workspots_app",
		`SELECT count(*) FROM information_schema.tables t
		 WHERE t.table_schema = 'public' AND t.table_name <> 'schema_migrations'
		   AND NOT has_table_privilege('workspots_app', format('%I.%I', t.table_schema, t.table_name), 'SELECT')`, 0)
	count("sequences workspots_app may setval",
		`SELECT count(*) FROM information_schema.sequences s
		 WHERE has_sequence_privilege('workspots_app', format('%I.%I', s.sequence_schema, s.sequence_name), 'UPDATE')`, 0)
	count("sequences without USAGE for workspots_app",
		`SELECT count(*) FROM information_schema.sequences s
		 WHERE NOT has_sequence_privilege('workspots_app', format('%I.%I', s.sequence_schema, s.sequence_name), 'USAGE')`, 0)
	count("security definer functions workspots_app may execute",
		`SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		 WHERE n.nspname = 'public' AND p.prosecdef AND has_function_privilege('workspots_app', p.oid, 'EXECUTE')`, 0)
	// A SECURITY DEFINER function created later by the migrate role must not be
	// executable by the app role either (rule 2: PUBLIC gets execute by default).
	if _, err := conn.Exec(ctx, `CREATE FUNCTION probe_secdef() RETURNS int LANGUAGE sql SECURITY DEFINER AS 'SELECT 1'`); err != nil {
		t.Fatal(err)
	}
	count("execute on a new security definer function for workspots_app",
		`SELECT count(*) FROM (SELECT 1 WHERE has_function_privilege('workspots_app', 'probe_secdef()', 'EXECUTE')) s`, 0)
	// At least one application table exists, so the checks above checked something (rule 9).
	var tables int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_name <> 'schema_migrations'`).Scan(&tables); err != nil || tables < 1 {
		t.Fatalf("application tables: %d, %v", tables, err)
	}
}

func TestApplyIsIdempotentAndOrdered(t *testing.T) {
	t.Parallel()
	adminURL, _ := testdb.New(t) // already migrated once
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var names []string
	rows, _ := conn.Query(ctx, `SELECT name FROM schema_migrations ORDER BY applied_at, name`)
	names, err = pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(names) == 0 {
		t.Fatalf("expected recorded migrations, got %v, %v", names, err)
	}
	if names[0] != "0001_init.sql" {
		t.Errorf("first migration recorded was %q", names[0])
	}
	again, err := migrate.Apply(ctx, conn, migrations.FS)
	if err != nil || len(again) != 0 {
		t.Errorf("second Apply applied %v, %v", again, err)
	}
}

// Migrations refuse to run unless the migrate role owns the database: REVOKE
// on the public schema by a non-owner is only a warning in Postgres.
func TestMigrationsRefuseADatabaseOwnedByAnotherRole(t *testing.T) {
	t.Parallel()
	adminURL, _ := testdb.New(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	name := "test_owned_" + strings.TrimPrefix(conn.Config().Database, "test_")
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name+" OWNER workspots_app"); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)") //nolint:errcheck
	cfg := conn.Config().Copy()
	cfg.Database = name
	other, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close(ctx)
	_, err = migrate.Apply(ctx, other, migrations.FS)
	if err == nil || !strings.Contains(err.Error(), "own") {
		t.Fatalf("expected an error about database ownership, got %v", err)
	}
}

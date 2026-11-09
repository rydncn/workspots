-- Creates the two database roles and the development databases if missing.
-- Run as a superuser, by scripts/dev-db.sh locally and by CI. In production an
-- operator creates the same two roles with real passwords (ADR 0004).
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'workspots_migrate') THEN
    -- CREATEDB: the acceptance tests create a database per test (ADR 0005).
    -- Development and CI only; the production migrate role has no such right.
    CREATE ROLE workspots_migrate LOGIN PASSWORD 'dev-migrate' CREATEDB;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'workspots_app') THEN
    CREATE ROLE workspots_app LOGIN PASSWORD 'dev-app';
  END IF;
  -- Development and CI only: lets the migrate role end the app role's
  -- connections when a test drops its database (DROP DATABASE ... WITH (FORCE)).
  IF NOT EXISTS (SELECT 1 FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid JOIN pg_roles g ON g.oid = m.member
                 WHERE r.rolname = 'workspots_app' AND g.rolname = 'workspots_migrate') THEN
    GRANT workspots_app TO workspots_migrate;
  END IF;
END $$;
SELECT 'CREATE DATABASE workspots_dev OWNER workspots_migrate'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'workspots_dev') \gexec
SELECT 'CREATE DATABASE workspots_test OWNER workspots_migrate'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'workspots_test') \gexec

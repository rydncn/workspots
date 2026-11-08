-- Schema ownership and the app role's grants (ADR 0004), then the spots table.
-- Runs as workspots_migrate, which owns everything it creates and must own
-- the database too: REVOKE on the public schema by anyone else is only a
-- warning in Postgres, which would leave PUBLIC with USAGE and nothing failing.
DO $$
BEGIN
  IF (SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname = current_database()) <> current_user THEN
    RAISE EXCEPTION 'migrations must run as the role that owns the database (%), not %',
      (SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname = current_database()), current_user;
  END IF;
END $$;

REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO workspots_app;

-- Every table and sequence this role creates from now on gets exactly these
-- grants. schema_migrations already exists, so it gets none.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO workspots_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE ON SEQUENCES TO workspots_app;
-- Global, not IN SCHEMA: a per-schema default ACL starts empty, so revoking
-- from it stores nothing and PUBLIC would keep execute on every new function.
ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

CREATE TABLE spots (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name          text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  kind          text NOT NULL CHECK (kind IN ('cafe', 'library', 'coworking', 'hotel_lobby', 'other')),
  area          text NOT NULL CHECK (length(area) BETWEEN 1 AND 80),
  address       text NOT NULL CHECK (length(address) BETWEEN 1 AND 200),
  opening_hours text NOT NULL DEFAULT '' CHECK (length(opening_hours) <= 500),
  status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
  created_by    bigint,  -- references users(id) once that table exists; null when the author is deleted (ADR 0011)
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX spots_status_area_idx ON spots (status, lower(area));

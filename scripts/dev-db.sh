#!/bin/sh
# Starts the local Postgres 16 cluster and creates the development roles and
# databases, idempotently. Run it at the start of every session: the
# environment installs Postgres, but the service doesn't survive the build.
#
#   scripts/dev-db.sh            # start, create what's missing, print the URLs
#
# Two roles, as in production: workspots_migrate owns the schema and runs the
# migrations; workspots_app owns nothing and gets only the grants its queries
# need (ADR 0004). The passwords below are development values; the app refuses
# to start in production mode with them (docs/design.md).
set -eu

CLUSTER="${PG_CLUSTER:-16 main}"
as_postgres() { su -s /bin/sh postgres -c "$*"; }

# shellcheck disable=SC2086
if ! pg_lsclusters -h | awk '{print $4}' | grep -qx online; then
  pg_ctlcluster $CLUSTER start
fi
for _ in 1 2 3 4 5 6 7 8 9 10; do
  as_postgres "pg_isready -q" && break
  sleep 1
done

sql() { as_postgres "psql -v ON_ERROR_STOP=1 -qAt -c \"$1\""; }
ensure_role() {
  [ "$(sql "SELECT 1 FROM pg_roles WHERE rolname = '$1'")" = 1 ] ||
    sql "CREATE ROLE $1 LOGIN PASSWORD '$2' $3"
}
ensure_db() {
  [ "$(sql "SELECT 1 FROM pg_database WHERE datname = '$1'")" = 1 ] ||
    sql "CREATE DATABASE $1 OWNER workspots_migrate"
}

# CREATEDB: the acceptance tests create a database per test from a template
# (ADR 0005). Development only; the production migrate role has no such right.
ensure_role workspots_migrate dev-migrate CREATEDB
ensure_role workspots_app dev-app ""
ensure_db workspots_dev
ensure_db workspots_test

cat <<URLS
Postgres is up.
  DATABASE_URL=postgres://workspots_app:dev-app@localhost:5432/workspots_dev
  MIGRATE_DATABASE_URL=postgres://workspots_migrate:dev-migrate@localhost:5432/workspots_dev
  TEST_DATABASE_URL=postgres://workspots_migrate:dev-migrate@localhost:5432/workspots_test
URLS

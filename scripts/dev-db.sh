#!/bin/sh
# Starts the local Postgres 16 cluster and creates the development roles and
# databases, idempotently. Run it at the start of every session: the
# environment installs Postgres, but the service doesn't survive the build.
#
#   scripts/dev-db.sh            # start, create what's missing, print the URLs
#
# Two roles, as in production: workspots_migrate owns the schema and runs the
# migrations; workspots_app owns nothing and gets only the grants its queries
# need (ADR 0004). scripts/db-init.sql holds the roles; its passwords are
# development values, and the app refuses to start in production mode with
# them (docs/design.md).
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

as_postgres "psql -v ON_ERROR_STOP=1 -q -f $(pwd)/scripts/db-init.sql"

cat <<URLS
Postgres is up.
  DATABASE_URL=postgres://workspots_app:dev-app@localhost:5432/workspots_dev
  MIGRATE_DATABASE_URL=postgres://workspots_migrate:dev-migrate@localhost:5432/workspots_dev
  TEST_DATABASE_URL=postgres://workspots_migrate:dev-migrate@localhost:5432/workspots_test
URLS

# Workspots

Good places to work remotely: cafés, libraries, coworking spaces, hotel lobbies. Search by area and by what matters for working; sign in to add spots and leave reports; moderators keep it honest. Server-rendered Go and Postgres, no JavaScript.

Built as trial 2 of the [ai-playbook](https://github.com/rydncn/ai-playbook): see `docs/design.md`, `docs/trial-log.md` and `docs/playbook-feedback.md`.

## Run it

```bash
scripts/dev-db.sh                                   # Postgres, roles and databases (prints the URLs)
MIGRATE_DATABASE_URL=postgres://workspots_migrate:dev-migrate@localhost:5432/workspots_dev go run ./cmd/workspots migrate
DATABASE_URL=postgres://workspots_app:dev-app@localhost:5432/workspots_dev go run ./cmd/workspots
```

Then open <http://127.0.0.1:8080/>. Everyone who signs up is a member. The first admin is made from the command line, and then sets roles at `/admin/users`:

```bash
MIGRATE_DATABASE_URL=postgres://workspots_migrate:dev-migrate@localhost:5432/workspots_dev go run ./cmd/workspots grant-role you@example.com admin
``` `go test ./...` runs every test against the same Postgres, each in its own database.

## Configuration

All from the environment (`internal/config`): `WORKSPOTS_ENV` (`development` or `production`), `ADDR`, `DATABASE_URL`, `BASE_URL`, `TRUSTED_PROXY`, `LOG_LEVEL`. In production the process refuses to start with a missing or development value. An operator creates the two database roles (`scripts/db-init.sql` shows them) with real passwords, creates the database **owned by `workspots_migrate`**, and runs `workspots migrate` as that role; the first migration refuses any other owner.

# Workspots

Finds good places to work remotely: cafés, libraries, coworking spaces, hotel lobbies. Anyone searches; members add spots and leave reports; moderators approve and hide. Server-rendered Go, Postgres, no JavaScript.

Set up against ai-playbook `f2fd445`. Its `CHANGELOG.md` lists what changed since, and what to do about each change. This is trial 2 of that playbook's core: feedback on it goes in `docs/playbook-feedback.md`, effort and defects in `docs/trial-log.md`.

Docs: design and acceptance criteria in `docs/design.md`, decisions in `docs/decisions/`, where the work is in `docs/handoff.md`. A slice's plan is its pull request description.

## Commands

```bash
scripts/dev-db.sh                 # Start Postgres and create the dev roles and databases. First, every session.
go run ./cmd/workspots migrate    # Apply migrations (needs MIGRATE_DATABASE_URL)
go run ./cmd/workspots            # Serve on 127.0.0.1:8080 (needs DATABASE_URL)
go test ./...                     # All tests ONCE, against the dev Postgres (each test makes its own database)
go test ./internal/web -run AC23  # One test
gofmt -l . && go vet ./... && staticcheck ./...   # Format, vet, lint
govulncheck ./...                 # Go dependency audit
cd e2e && npm ci && npx playwright test           # End-to-end and accessibility (from slice 2)
```

Development URLs are printed by `scripts/dev-db.sh`. Commands that reach a shared environment ask first in `.claude/settings.json`; never work around that.

At the start of a session, if the plugin check reports missing plugins, tell the user before starting work rather than doing their steps by hand.

## Architecture

`cmd/workspots` is the one binary: `serve` (default) and `migrate`. Under `internal/`:

- `web`: the only package that imports `net/http`. Routes, handlers, `html/template` pages, middleware (correlation ID, security headers, request log, recover, sessions, CSRF, rate limits), and `problems.go`, where every domain error is mapped to its RFC 9457 problem once.
- `spots`, `accounts`, `moderation`: domain code and plain SQL through `pgx`. They return named errors and know nothing about HTTP.
- `migrate` applies `migrations/*.sql` in order; `config` reads the environment; `obs` is the logger, correlation ID and query tracer; `testdb` gives each test its own migrated database.

Request flow: `correlate` → `securityHeaders` → `logRequests` → `recover` → `routeErrors(mux)` → handler → domain → `pgx` as `workspots_app`.

### Security boundaries

- **Only `internal/web` touches HTTP.** Checked by CI's "HTTP stays in web" grep over `internal/` imports.
- **The app connects as `workspots_app`, which owns nothing** and has only SELECT/INSERT/UPDATE/DELETE on application tables and USAGE on sequences; `workspots_migrate` owns the schema. Checked by `TestAC30AppRolePrivileges` on an empty migrated database. A new table gets its grants from the default privileges set in `0001_init.sql`.
- **Every error response is a problem** (JSON, or the same fields as an HTML page for browsers), including the router's 404 and 405; a 500 reveals only the correlation ID. Checked by `TestAC23`.
- **Security headers on every response**, CSP without `unsafe-inline`; cookies `__Host-`, `Secure`, `HttpOnly`, `SameSite=Lax`; CSRF on every `POST` (HMAC of the session ID keyed by a `__Host-csrf` secret, plus same-origin `Origin`/`Sec-Fetch-Site`). Checked by `TestAC24` and, from slice 3, `TestAC17`.
- **Configuration from the environment only.** Production refuses missing or development values and names the variable, never the value. Checked by `TestAC27`.
- **Nothing personal or secret in a log line.** Log fixed messages with fields; never request bodies, headers, query strings, addresses, notes or tokens. The query tracer logs SQL text, never arguments. Checked from slice 3 by `TestAC26`.
- **One correlation ID per request**, carried in `context.Context` into every log line, query and goroutine. Checked by `TestAC25`.

## Conventions

- Go 1.27, standard library first (ADR 0001). A new module needs an ADR saying why.
- Test-first: write the test, see it fail on its assertion, then the code. Acceptance tests are named `TestAC<n>…` for the criterion in `docs/design.md`, run the real server over HTTP against a real database from `testdb.New`, and call `t.Parallel()`.
- Templates: `html/template` only, one file per page sharing `layout.html`; never `template.HTML` on user data.
- SQL: plain, in the domain package that owns the table, parameters only. Migrations are `migrations/NNNN_name.sql`, never edited once merged.
- British English in copy, comments and commits. Conventional commits (`feat:`, `fix:`, `docs:`, `ci:`, `test:`, `chore:`).

## Area rules

These load automatically when the agent works on matching files:

- `.claude/rules/api.md`: endpoints, problem details, domain errors, cookies, CSRF, rate limits, passwords, redirects.
- `.claude/rules/observability.md`: structured logging and correlation IDs.

## Workflow

- Non-trivial work: the design first, then a draft pull request whose description is the plan: the criteria it satisfies and how each is observed, and whether it is a one-way door (migrations, deletion, auth, public URLs, agent configuration, new dependencies, release settings) or a two-way door, naming the one-way files.
- A task is done when its effect has been observed, not when the code has been read back.
- Before marking a slice ready: a fresh-context review by a subagent given only the diff, `docs/design.md` and this file. Fix findings test-first or record why not. After slice 3 and at the end: a whole-codebase review briefed from this file and the design, not a diff. `/security-audit` once before the final review.
- Every gate is broken on purpose once and seen to fail; the pull request records it.
- When the agent gets something wrong, record the lesson here or in the matching rules file. A rule broken twice becomes a check.

## Lessons

- `DROP DATABASE … WITH (FORCE)` needs the right to end the other role's connections: dev and CI grant `workspots_app` to `workspots_migrate` in `scripts/db-init.sql`. Production never drops databases.
- `has_table_privilege` with a bare name from `information_schema` resolves against the wrong schema; qualify it with `format('%I.%I', …)`.

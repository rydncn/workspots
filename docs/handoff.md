# Handoff

**Updated:** 2026-11-09, end of session 1 (design, slices 1 to 4). The session ran far past the context budget at Ryan's request; the next one must be fresh.

## Where the work is

- Slices 1 to 3 are on `main` (PRs #1, #4, #8; #5 landed on the slice 2 branch by mistake and #8 carried it over). Slice 4 (contribute) is PR #9 from `slice-4-contribute`, ready and green, fresh-context review fixed and recorded.
- `docs/trial-log.md` has slices 0 to 4; `docs/playbook-feedback.md` has 26 items. Commit the log before planting breaks or branching for anything disposable; stacked PRs cost a round trip here, so wait for the base to merge.
- `scripts/dev-db.sh` first in every session; then `go test ./...` and `cd e2e && npm ci && npm test` work with no further setup. e2e signs in as seeded members, readers and contributors (`e2e/seed.sql`); only one journey signs up; the report journeys use the seeded "Reportable Café" so they cannot move the attributes the browse journeys assert on.

## Next: slice 5, moderation and roles

Criteria: AC18, AC19, AC20, AC21, and AC6's moderator exception (already in `spots.visible` via `Viewer.Moderator`; test it).

What it needs: `internal/moderation`: `PendingSpots`, `Approve`, `Reject` (status → rejected, author sees "rejected" on their account page and spot page, AC19), `HideReport(reportID, reason)` (hidden flag; AC20 says the queue shows it hidden, so keep a `hidden_reason`/`hidden_at` on reports via migration 0005), `OpenProblems` (the queue: spots, reports and problem reports together), `Resolve` for a problem report, and `accounts.SetRole(adminID, userID, role)` with `ErrPermissionDenied` unless the actor is an admin (AC21). `cmd/workspots grant-role <email> <role>` to bootstrap the first admin (reads `MIGRATE_DATABASE_URL` or `DATABASE_URL`; document in the README; this is how an operator makes the first admin). `internal/web`: `GET /moderation` (403 problem for members, 401 for visitors: AC18; `requireRole` helper beside `requireUser`), `POST /moderation/spots/{id}/approve|reject`, `POST /moderation/reports/{id}/hide`, `POST /moderation/problems/{id}/resolve`, `GET/POST /admin/users` for roles (admin only; moderators get 403, AC21). Every moderation route has a test where a member and a moderator are refused where they should be; the AC30 test needs nothing new unless a table is added. e2e: seed a moderator and an admin (same password hash), journeys for approve, hide and change role with axe; keyboard once. Remember the limiter: moderators' actions need no limit.

Then: fresh-context review, trial log, feedback, PR description as the plan. Door: one-way (authorisation, roles, migration).

Slice 6 after that: AC11 account deletion (everything personal gone, approved spots anonymised, proved in the database), the Dockerfile and release rehearsal (Caddy with its internal CA, as the design's rehearsal section says), `/security-audit`, the final whole-codebase review, and the trial's totals and comparison in `docs/trial-log.md`.

## Environment notes

- Docker daemon doesn't run in this container: the Dockerfile and rehearsal can only be verified in CI (slice 6).
- npm was upgraded to 11 globally; `staticcheck` and `govulncheck` are installed in `$(go env GOPATH)/bin`. gitleaks and zizmor release downloads from github.com work.
- The harness floor is about 80k tokens of context; the budget hook will warn early. Record peak context per session in `docs/trial-log.md`.

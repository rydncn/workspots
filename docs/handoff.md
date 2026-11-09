# Handoff

**Updated:** 2026-11-09, end of session 1 (design, slices 1 to 3). The session ran far past the context budget at Ryan's request; the next one must be fresh.

## Where the work is

- Slice 1 merged (PR #1), slice 2 merged (PR #4). Slice 3 (PR #5) merged into the slice 2 branch rather than `main`, because #4 had merged first; PR #6 carries it to `main`. Check `git log origin/main` shows `0003_accounts.sql` before starting slice 4. Slice 3 was with its fresh-context review, the whole-codebase review after slice 3 and Ryan's outside review (open redirect via a tab, rate-limit race, uncapped argon2id, one visibility rule, search paging) all fixed and recorded on the PR and in the trial log.
- Slice 4 note: `spots.visible` is the one place that says which spots a visitor may see; extend it (viewer id for the author, role for moderators) rather than adding conditions to a query. Review briefs for anything touching auth should ask by name: what does the browser do to this value, what happens when twenty of these arrive at once, what does one of these cost.
- `docs/trial-log.md` has slices 0 to 3; `docs/playbook-feedback.md` has 25 items. Commit the log before planting breaks or branching for anything disposable.
- `scripts/dev-db.sh` first in every session; then `go test ./...` and `cd e2e && npm ci && npm test` work with no further setup. e2e signs in as seeded members (`e2e/seed.sql`); only one journey signs up, because sign-up is rate limited per IP.

## Next: slice 4, contribute

Criteria: AC13, AC14, AC15, AC16, AC22, plus AC6's author exception (a pending spot is visible to its author, marked awaiting approval).

What it needs: migration 0004 with `problem_reports` (target spot or report, reporter, reason, status open/resolved, created_at; reporter FK on delete cascade). `internal/spots`: `Add` (name, kind, area, address, opening hours, declared attributes; status pending; `created_by`), `AddReport` (ratings, note up to 500 characters, author), `Get` taking the viewer so an author sees their pending spot, "my spots" for the account page. `internal/moderation` (or in `spots`): `ReportProblem`. `internal/web`: `GET/POST /spots/new`, `GET/POST /spots/{id}/reports/new`, `GET/POST /spots/{id}/problems/new` and `/reports/{id}/problems/new`; `requireUser` already redirects GETs to sign-in with a safe `next` and refuses POSTs with a 401 problem (AC13); validation with the existing `invalid` helper and form-errors partial (AC15); the account page lists the member's spots with their status. Templates for the three forms with every field labelled; the report form's ratings as radio groups in fieldsets. e2e: the add-spot, report and problem journeys with axe in the validation-failure state and at 320 pixels, by keyboard once; use the seeded members. Gates: break the author-visibility check and the note-length check once.

Then: fresh-context review, trial log, feedback, PR description as the plan. Door: one-way (migration, authorisation of who may add and report).

## Environment notes

- Docker daemon doesn't run in this container: the Dockerfile and rehearsal can only be verified in CI (slice 6).
- npm was upgraded to 11 globally; `staticcheck` and `govulncheck` are installed in `$(go env GOPATH)/bin`. gitleaks and zizmor release downloads from github.com work.
- The harness floor is about 80k tokens of context; the budget hook will warn early. Record peak context per session in `docs/trial-log.md`.

# Handoff

**Updated:** 2026-11-09, session 2 (slice 5).

## Where the work is

- Slices 1 to 4 are on `main` (PRs #1, #4, #8, #9). Slice 5 (moderation and roles) is PR #10 from `ccr-20627409-4xiipi`: draft, fresh-context review done and recorded in `docs/trial-log.md`. The CI test and e2e jobs run only once the PR is marked ready for review, so mark it ready and watch that run before merging.
- `docs/trial-log.md` has slices 0 to 5; `docs/playbook-feedback.md` has 29 items.
- `scripts/dev-db.sh` first in every session; then `go test ./...`. For e2e: `cd e2e && npm ci && PW_CHROMIUM=/opt/pw-browsers/chromium npx playwright test` in a container whose Playwright download is blocked (the config takes `PW_CHROMIUM`); plain `npm test` where it is not. e2e signs in as seeded members, readers, contributors, moderators, admins and members to promote (`e2e/seed.sql`), one of each per Playwright project so the two projects never share state; the accounts journey changes the seeded *member*'s password, so other journeys sign in as a contributor.
- The first admin is made with `workspots grant-role <email> <role>` (README); after that `/admin/users` sets roles by email. A role change signs the account out. An admin can demote themselves, including the last admin; `grant-role` is the way back.

## Next: slice 6

AC11 account deletion (everything personal gone, approved spots anonymised, proved in the database; ADR 0011), the Dockerfile and release rehearsal (Caddy with its internal CA, as the design's rehearsal section says), `/security-audit`, the final whole-codebase review briefed from `CLAUDE.md` and the design, and the trial's totals and comparison in `docs/trial-log.md`. Hidden reports keep `hidden_reason` and `hidden_at` (migration 0006); deletion cascades reports, so nothing new to anonymise there.

## Environment notes

- Docker daemon doesn't run in this container: the Dockerfile and rehearsal can only be verified in CI (slice 6).
- These notes are about one container and may not survive to the next. Session 2 had to `go install` `staticcheck` and `govulncheck` (the latter cannot reach vuln.go.dev here; CI runs it), and the pinned Playwright could not download its browser: use the pre-installed one via `PW_CHROMIUM`.
- The harness floor is about 80k tokens of context; the budget hook will warn early. Record peak context per session in `docs/trial-log.md`.

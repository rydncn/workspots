# Handoff

**Updated:** 2026-11-10, session 3 (slice 6, the last).

## Where the work is

- Slices 1 to 5 are on `main` (PRs #1, #4, #8, #9, #10). Slice 6 (account deletion, the production image and its release rehearsal, the security audit, the whole-codebase review, the trial's totals) is PR #11 from `claude/workspots-slice-6-setup-ns9wy2`. Its description is the plan and records the gate breaks; `docs/trial-log.md` has the slice entry and the totals.
- The trial is complete once PR #11 merges. Three things for a person:
  1. **Merge PR #11** on green CI (ADR 0010). The test, end-to-end and release jobs run once it is ready for review; `workflow_dispatch` runs on the branch (47, 50 red on purpose, and the final one) already showed them green.
  2. **Add `Release rehearsal` to the required checks** in the `main` ruleset: branch protection requires jobs by name, and this slice added one.
  3. **Production needs `TRUSTED_PROXY`** set to the address the TLS proxy connects from (AC27 now refuses to start without it) and `ADDR` set when the binary runs in a container (it defaults to loopback).
- Open leads from the security audit are in `docs/security-audit.md`: both were fixed in this slice; the two coverage units the audit's critic deferred (the migration runner's privilege model; Go `strings.ToLower` versus Postgres `lower(email)` in the per-address limiter key) are the first work of any later audit. Hardening notes recorded but not acted on are listed in the trial log's slice 6 entry.
- The playbook branch `claude/workspots-slice-6-setup-ns9wy2` on `rydncn/ai-playbook` carries this trial's backlog items (through slice 6); no pull request was opened for it.

## Running it

- `scripts/dev-db.sh` first in every session; then `go test ./...`. For e2e: `cd e2e && npm ci && PW_CHROMIUM=/opt/pw-browsers/chromium npx playwright test` in a container whose Playwright download is blocked; plain `npm test` where it is not. The seeded accounts (`e2e/seed.sql`) are one per Playwright project; the accounts journey changes the seeded *member*'s password and the deletion journey deletes the seeded *deletable* account, so other journeys sign in as a contributor.
- The first admin is made with `workspots grant-role <email> <role>` (README); after that `/admin/users` sets roles by email. A role change signs the account out.
- The image: `docker build -t workspots .`; CI's release job is the reference for running it (README, Container). Docker does not run in the agent's cloud container, so image changes are verified in CI only.

## Environment notes (this container only)

- Session 3 had to `go install` `staticcheck` and `govulncheck` (the latter cannot reach vuln.go.dev here; CI runs it) and use the pre-installed Chromium through `PW_CHROMIUM`. The container registries and `git ls-remote` to GitHub were reachable, so image digests and action commit SHAs could be looked up; `api.github.com` was not.
- The context-budget hook fires on user prompts only, so an autonomous session with one prompt sees no budget figure (playbook feedback 34).

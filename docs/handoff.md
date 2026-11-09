# Handoff

**Updated:** 2026-11-08, end of session 1 (design and slice 1).

## Where the work is

- Slice 1 is built, reviewed and green in CI on PR #1 (`slice-1-walking-skeleton`), ready for Ryan to review and merge; it is a one-way door (migrations, agent configuration, the error and header contract). The fresh-context review's ten findings are fixed. Branch protection is on `main` (ADR 0010); what the API showed is recorded in a comment on PR #1.
- `docs/trial-log.md` has slice 0 and slice 1 (criteria, context, wall clock, defects by layer, gate breaks); `docs/playbook-feedback.md` has 19 items so far. Keep both current per slice; the slice 1 entry was nearly lost to a `git add -A` on a throwaway branch, so commit the log before branching for anything disposable.
- `scripts/dev-db.sh` works: run it first in every session. `go test ./...` then passes with no further setup.

## Next: slice 2, browse and search

Criteria: AC2, AC3, AC4, AC5, AC6, and the harness for AC28 and AC29 (every later slice adds its pages to it). Branch from `main` once PR #1 is merged (or from `slice-1-walking-skeleton` if not yet, and rebase).

What it needs: `GET /search` with `area` and the filter fields (Wi-Fi at least, power, noise at most, no time limit, step-free, accessible toilet, quiet space), a `reports` table with the attribute ratings and `hidden` flag plus the spot's declared attributes (migration 0002: remember grants come from default privileges, and the AC30 test will tell you), `GET /spots/{id}` with attributes (median of visible reports, else declared), opening hours and dated visible reports newest first, 404 problem `spot-not-found` for unknown or pending spots (author and moderators excepted: until accounts exist, "everyone" is 404). Templates for search and spot pages, with worst-case data in mind (long names, unbreakable words, zero results).

`e2e/` with the playbook's `templates/npmrc` as `.npmrc`, `package.json` with exact versions of `@playwright/test` and `@axe-core/playwright` (local npm is 11; `min-release-age` applies), Playwright config with `javaScriptEnabled: false` as the default project, a 320-pixel-wide project, axe against WCAG 2.2 AA on every page in every state reached, keyboard-only journeys, and a no-horizontal-scroll check. A `globalSetup` that builds and starts the binary against a fresh test database (`testdb` logic, or `scripts/dev-db.sh` plus `workspots migrate`). Playwright and Chromium are preinstalled in this environment; do not run `playwright install` (see the environment notes). Add to CI: the end-to-end and accessibility job (waits for ready-for-review like Tests), and in the security job lockfile-lint and `npm audit signatures` for `e2e/` plus a Dependabot `npm` block for `/e2e`. Break each new gate once.

Then: fresh-context review, trial log, feedback, PR description as the plan. Door: two-way except the migration and the CI changes.

## Environment notes

- Docker daemon doesn't run in this container: the Dockerfile and rehearsal can only be verified in CI (slice 6).
- npm was upgraded to 11 globally; `staticcheck` and `govulncheck` are installed in `$(go env GOPATH)/bin`. gitleaks and zizmor release downloads from github.com work.
- The harness floor is about 80k tokens of context; the budget hook will warn early. Record peak context per session in `docs/trial-log.md`.

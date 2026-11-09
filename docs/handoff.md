# Handoff

**Updated:** 2026-11-09, still session 1 (design, slice 1, slice 2).

## Where the work is

- Slice 1 merged (PR #1). Slice 2 (browse and search, AC2 to AC6, the AC28/AC29 harness) is on PR #3 from `slice-2-browse-and-search`: Go acceptance tests, `e2e/` with Playwright and axe, the e2e CI job and the npm supply-chain steps. See the PR description for what was observed and broken.
- `docs/trial-log.md` has slices 0 to 2; `docs/playbook-feedback.md` has 22 items. Commit the log before planting breaks or branching for anything disposable.
- `scripts/dev-db.sh` first in every session; then `go test ./...` and `cd e2e && npm ci && npm test` work with no further setup.

## Next: slice 3, accounts

Criteria: AC7, AC8, AC9, AC10, AC12, AC17, AC26, and the goroutine clause of AC25 (the welcome mail runs in a goroutine with a context derived from the request). Whole-codebase review after this slice (CLAUDE.md "Workflow").

What it needs: migration 0003 with `users` (id, email unique case-insensitively, password_hash, role member/moderator/admin, created_at), `sessions` (sha256 of the token, user_id, created_at, expires_at), and the `reports.author_id` foreign key. `internal/accounts`: sign-up (12 to 1,024 bytes, argon2id via `golang.org/x/crypto/argon2`, ADR 0012, one new module), sign-in with the same message for wrong password and unknown address, session create/rotate/end, password change ending other sessions, the dev mail outbox (`internal/mail`, ADR 0009: a `Sender` interface, a dev outbox writing plain text to its own stream, a recording fake for tests). `internal/web`: sign-up, sign-in, sign-out, account and change-password pages and handlers; session middleware (`__Host-session`, Secure, HttpOnly, SameSite=Lax); CSRF as designed (`__Host-csrf` secret cookie, HMAC of the session ID or `anonymous`, rotated at sign-in, plus Origin/Sec-Fetch-Site on every POST); rate limits in memory per address and per IP (AC12) using `TRUSTED_PROXY` for the IP; the no-personal-data log test (AC26) capturing the structured log across sign-up, failed sign-in, a report note and a problem report (the last two can be seeded). Add the new pages and states (validation failure, signed in, signed out) to the e2e suite and axe; add a signed-in keyboard journey. Rule files: `.claude/rules/api.md` already describes all of this.

Then: fresh-context review, then the whole-codebase review (briefed from `CLAUDE.md` and `docs/design.md`, not a diff), trial log, feedback. Door: one-way (auth, migration, new dependency).

## Environment notes

- Docker daemon doesn't run in this container: the Dockerfile and rehearsal can only be verified in CI (slice 6).
- npm was upgraded to 11 globally; `staticcheck` and `govulncheck` are installed in `$(go env GOPATH)/bin`. gitleaks and zizmor release downloads from github.com work.
- The harness floor is about 80k tokens of context; the budget hook will warn early. Record peak context per session in `docs/trial-log.md`.

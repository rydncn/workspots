# Handoff

**Updated:** 2026-11-08, end of the design session (session 1).

## Where the work is

- The design and its 30 acceptance criteria in `docs/design.md` are approved by Ryan, with five amendments made after review: pre-session CSRF for sign-in and sign-up (AC17 and "Shape"), session rotation at sign-in (AC8), a 1,024-byte password maximum (AC7), HTTPS in the release rehearsal through a Caddy TLS proxy (new section), and branch protection after slice 1 (ADR 0010).
- `main` holds the first commit (`f853267`); this branch holds the amendments on top. Slice 1 starts from this branch, so its pull request carries the amendments to `main`.
- `scripts/dev-db.sh` works: run it first in every session.

## Next: slice 1, walking skeleton and gates

Criteria: AC1, AC23, AC24, AC25, AC27, AC30. Also: `go.mod`, `cmd/workspots`, `internal/{web,config,migrate}`, the first migration with the app role's grants, `CLAUDE.md` (under 200 lines, recording playbook commit `f2fd445`), `.claude/rules/api.md` and `observability.md` rewritten for Go, `.claude/settings.json` from the playbook template trimmed to Go commands, `.claude/hooks/` (Go versions of after-edit and before-stop; context-budget and check-plugins copied), `.github/CODEOWNERS` with `@rydncn`, `.github/workflows/ci.yml` adapted to Go, `.github/dependabot.yml` with gomod, github-actions, npm (`/e2e`) and docker blocks.

Order, from the playbook: gates before much code. Test-first for every behaviour, each test seen failing first. Break each gate once and record it in the PR. Open the PR as a draft; its description is the plan (criteria and how each is observed) and says it is a one-way door (migrations, auth plumbing, agent configuration, new dependencies: `pgx`, `x/crypto`).

**Stop after the first full CI run** and ask Ryan to turn on branch protection (ADR 0010), then read it back from the API.

## Environment notes

- Docker daemon doesn't run in this container: the Dockerfile and rehearsal can only be verified in CI (slice 6).
- npm was upgraded to 11 globally; `staticcheck` and `govulncheck` are installed in `$(go env GOPATH)/bin`. gitleaks and zizmor release downloads from github.com work.
- The harness floor is about 80k tokens of context; the budget hook will warn early. Record peak context per session in `docs/trial-log.md`.

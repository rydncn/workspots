# Trial log: Workspots under the ai-playbook core

Trial 2 of the [ai-playbook](https://github.com/rydncn/ai-playbook) at commit `f2fd445`, applying only the core ("Start here") to a stack the playbook wasn't written for: Go, server-rendered HTML, Postgres, npm only for `e2e/`. Compared at the end with trial 1 (a small Expo app under the full playbook: stronger on all four review axes, 9 findings, two to three times the effort per behaviour).

How context is measured: the playbook's context-budget hook reads the transcript's last usage figure. In this cloud environment the harness's own system prompt and tool definitions are about 80k tokens before any work, so every figure below includes that floor.

## Per slice

### Slice 0: design and setup (this log, the design, ADRs, `scripts/dev-db.sh`)

- Criteria delivered: none (design only).
- Sessions: 1. Peak context: hook not yet installed; about 160k reported by the harness at the end, of which about 80k is the floor. Wall clock: about 1.5 hours including Ryan's review.
- Defects found: 4 by the human design review (login CSRF uncovered, session rotation missing from AC8, no maximum password length, rehearsal HTTPS versus AC27). None by tooling: nothing to test yet.

<!-- Template for each slice:
### Slice N: <name> (PR #)
- Criteria delivered: AC…
- Sessions: n. Peak context per session: …. Wall clock: ….
- Defects by layer:
  - tests (test-first): …
  - hooks: …
  - CI: …
  - fresh-context review: …
  - whole-codebase review: …
  - security audit: …
-->

## Totals and comparison

Filled in at the end.

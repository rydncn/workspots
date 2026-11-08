# Playbook feedback from trial 2 (Workspots: Go, HTML, Postgres)

Where the playbook core at `f2fd445` was wrong, slow, missing something or assumed npm and TypeScript, and the reading taken. The playbook itself is not changed here.

## Reading and setup

1. **"Start here" is short and clear, but the core still points at npm-shaped artefacts.** The CI template, the hooks (`after-edit.mjs`, `before-stop.mjs`, `pre-push`) and `settings.json` all key off `package.json` scripts, Prettier and ESLint. For Go every one has to be rewritten rather than copied. Reading taken: keep the hook *behaviour* (format and vet after each edit, fast checks before stopping, plugin check, context budget) and reimplement each as a small Node script that runs `gofmt`, `go vet` and `go test` instead. `context-budget.mjs` and `check-plugins.mjs` are stack-neutral and copied as they are.
2. **`docs/new-project.md` is the only ordered checklist, and it isn't marked as core or not.** Steps 1 to 7 mix core items (CI, hooks, `CLAUDE.md`, settings, CODEOWNERS) with "add as it grows" ones (`docs/plans/`, `docs/handoff.md`, rule checks, the Claude review workflow). Reading taken: followed the README's core list and used `new-project.md` only for ordering.
3. **The context budget is unreachable in a cloud session.** The harness's system prompt and tool definitions here are roughly 80k tokens before the first message, so a 100k budget leaves about 20k of working room. Rule 17's figure is "a practitioner's threshold", but the hook measures total context, so in this environment it would warn almost at once. Reading taken: record peak context per session as the hook reports it, and treat the budget as "about 100k of *work* above the floor" (so roughly 180k reported) for deciding when to hand off. Noted so the comparison with trial 1 is honest.
4. **npm 11.10 or later is assumed; this environment had 10.9.9.** The `.npmrc` template needs it for `min-release-age`, and the CI template checks it, but `new-project.md` only says so in passing. Upgraded npm globally before the first install.
5. **The design template fits a Go project unchanged.** `DESIGN.md`, `ADR.md` and the given/when/then criteria needed no adaptation.
6. **`api.md` assumes JSON clients.** "Every error response is `application/problem+json`" leaves a browser user looking at raw JSON for a 404. Reading taken: the problem is still the canonical error; a request whose `Accept` prefers HTML gets the same fields rendered as a page. The template could say this in its browser section.
7. **`api.md` says failure messages mustn't reveal whether an account exists, but gives no pattern for sign-up.** A sign-up form that says "address taken" reveals it. Reading taken (AC7): the same page for both cases, with the difference only in the mail that would be sent.
8. **Rule 14 versus "show what would be sent in the development log".** A development mail outbox that prints the recipient is personal data in a log line. Reading taken: the dev outbox is its own plain stream, separate from the structured log, and the no-personal-data test runs against the structured log.

## As the work goes

9. **The design review by a person found four gaps the template's prompts didn't ask about:** login CSRF (tokens "tied to the session" when there is no session yet), session rotation at sign-in (in `api.md` but not surfaced as a criterion), a maximum password length so argon2id can't be used for denial of service, and how a local release rehearsal provides HTTPS when production mode insists on it. `api.md` could carry the first three as one-liners; `GATES.md`'s release row could mention the TLS proxy.
10. **A handoff file was needed after all.** The design session passed the context budget before slice 1 started (the harness floor is about 80k; see item 3), so `docs/handoff.md` was written to carry "design approved, start slice 1" into a fresh session. The task asked for a reason: the budget, not the size of the work.

## Review layers

- The Claude PR review workflow (`claude-review.yml`) was not set up: it needs purchased API credit this account doesn't have. The fresh-context subagent review of each slice carried that layer.

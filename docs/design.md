# Workspots design

**Status:** draft · **Date:** 2026-11-08

## Goal

Workspots helps people find good places to work remotely: cafés, libraries, coworking spaces, hotel lobbies. Anyone can search spots by area and by what matters for working; signed-in members add spots and leave dated reports; moderators keep the catalogue honest. Out of scope: maps, geolocation, email delivery, hosting, payments, photos.

Every core journey works without JavaScript. The first version ships no JavaScript at all; any later script may only improve a journey that already works without it.

## Decisions

| Decision                                                           | ADR                                                   |
| ------------------------------------------------------------------ | ----------------------------------------------------- |
| Go standard library first: `net/http`, `html/template`, `log/slog`  | [0001](decisions/0001-go-standard-library-first.md)   |
| `pgx` for Postgres                                                 | [0002](decisions/0002-pgx-for-postgres.md)            |
| Plain SQL migrations applied by a small runner of our own          | [0003](decisions/0003-plain-sql-migrations.md)        |
| Two database roles: a migrator that owns the schema, an app role that owns nothing | [0004](decisions/0004-least-privilege-database-role.md) |
| Acceptance tests at tier 1 in Go (a database per test) and tier 2 with Playwright; no tier 3 | [0005](decisions/0005-acceptance-test-tiers.md) |
| Vanilla HTML and CSS, no build step, no JavaScript in v1           | [0006](decisions/0006-vanilla-html-and-css.md)        |
| Playwright and axe-core for end-to-end and accessibility tests     | [0007](decisions/0007-playwright-and-axe.md)          |
| Distroless image pinned by digest, non-root, no shell              | [0008](decisions/0008-container-image.md)             |
| Email is a development outbox; a real provider is a later decision | [0009](decisions/0009-email-outbox.md)                |
| Merging: a person merges on green CI; the agent asks               | [0010](decisions/0010-merge-policy.md)                |
| Account deletion removes everything personal; approved spots stay, anonymised | [0011](decisions/0011-account-deletion-scope.md) |
| argon2id from `golang.org/x/crypto`                                | [0012](decisions/0012-argon2id.md)                    |

## Shape

One Go binary, `cmd/workspots`, serving server-rendered HTML over `net/http`, talking to Postgres through `pgx`. Packages under `internal/`:

- `web`: routes, handlers, templates, middleware (correlation ID, security headers, sessions, CSRF, rate limits, problem details). Handlers call the domain and render templates; nothing else touches `http`.
- `spots`, `accounts`, `moderation`: domain code and SQL. Each returns named domain errors (`ErrSpotNotFound`, `ErrPermissionDenied`, …) that know nothing about HTTP; `web/problems.go` maps each to one problem type and status.
- `migrate`: applies `migrations/*.sql` in order, recording each in `schema_migrations`.
- `config`: reads the environment; in production mode refuses to start with a missing or development value.

Data: `users` (id, email, password_hash, role, created_at), `sessions` (hashed token, user, expires), `spots` (name, kind, area, address, opening hours, status pending/approved/rejected, created_by nullable), `reports` (spot, author, ratings per attribute, note, hidden flag, created_at), `problem_reports` (target spot or report, reporter, text, status). Spot attributes shown on a spot are the medians of its visible reports, plus the attributes the adder declared until the first report arrives.

Errors: every error response is an RFC 9457 problem. A request whose `Accept` prefers `text/html` gets the same problem rendered as an HTML page with the same fields; anything else gets `application/problem+json`. Both carry the correlation ID.

CSRF: one mechanism before and after sign-in. A `__Host-csrf` cookie (`Secure`, `HttpOnly`, `SameSite=Lax`) holds a random secret, issued with any form when absent; the form's hidden token is an HMAC-SHA256 of the session ID (or `anonymous` before sign-in) keyed by that secret, so the token is tied to the session once one exists and to the browser before. Both cookies rotate at sign-in. Independently, every `POST` must carry a same-origin `Origin` or `Sec-Fetch-Site: same-origin`.

Email: the app composes messages (welcome, password changed, account deleted) and hands them to a sender. The development sender writes them to a plain "dev outbox" stream, separate from the structured log, so the structured log never carries an address.

What is real in tests: the real server, the real Postgres with every migration applied, real sessions and password hashing. Faked at the edge: the mail sender (a recording fake), the clock and the request IP. Nothing else.

### Security boundaries

- Only `internal/web` imports `net/http`; domain packages have no notion of requests or cookies. Checked by a CI grep on imports.
- All SQL runs as `workspots_app`, which owns nothing and holds only the grants its queries need; a test queries the catalogue after migrating an empty database (AC30).
- Every state change is a `POST` with a CSRF token tied to the session and a same-site `Origin`/`Sec-Fetch-Site`; `GET` never changes state (AC17).
- Authorisation is checked per resource in the domain: who may approve, hide, change a role, or delete an account (AC18–AC21, AC11).
- Configuration and secrets come from the environment only; production mode refuses development values (AC27). Nothing personal or secret reaches a log line (AC26).

### Personal data and retention

| Data                                            | Why we hold it                      | Retention                                                  |
| ----------------------------------------------- | ----------------------------------- | ---------------------------------------------------------- |
| Email address, password hash                    | Sign in                             | Until the account is deleted                               |
| Sessions (hashed token, user id, expiry)        | Stay signed in                      | 30 days from creation, or sign-out, or account deletion    |
| Reports and problem reports (author, note, date)| The product                         | Deleted with the account                                   |
| Spots added (authorship)                        | Moderation and attribution          | Authorship removed when the account is deleted; a pending or rejected spot is deleted with it |
| Rate-limit counters (hashed IP and account key) | Abuse control                       | In memory, 15 minutes                                      |
| Logs (correlation ID, route, status, user id)   | Operations                          | Whatever the log sink keeps; never an address, note or token |

## Acceptance criteria

Numbered, one block per behaviour. Each becomes an acceptance test that names its number. "Problem" means an RFC 9457 problem response in the shape of the request's `Accept` header.

### Browse and search

**AC1. The home page lists approved spots only.** Given approved spots and a pending spot, when anyone requests `/`, then the approved spots are listed by name and the pending spot is absent.

**AC2. Search by area.** Given approved spots in "Leeds" and "LS6" and one in "York", when anyone searches for `leeds`, then only the Leeds spot is listed; `ls6` lists only the LS6 spot; the search text is kept in the form.

**AC3. Filters combine.** Given spots with different attributes, when a search asks for Wi-Fi at least "good", power sockets, noise at most "moderate", no seating time limit, step-free access, an accessible toilet or a quiet space, then only spots meeting every chosen filter are listed.

**AC4. An empty result is a page, not an error.** Given no matching spot, when anyone searches, then the page says no spots match, keeps the search and filters, and returns 200.

**AC5. A spot page shows its details and visible reports.** Given an approved spot with a hidden and a visible report, when anyone opens it, then its attributes, opening hours and the visible report with its date are shown, newest first, and the hidden report is not.

**AC6. An unknown spot is a problem, not a crash.** Given no spot with that id, when anyone opens it, then the response is a 404 problem with type `spot-not-found` and a correlation ID; a pending spot is 404 to everyone but its author and moderators.

### Accounts

**AC7. Sign up.** Given no account for the address, when someone signs up with an email and a password of at least 12 characters, then an account with role `member` exists and the dev outbox holds a welcome message to that address. Given an account already exists for the address, then no account is created, the dev outbox holds a "you already have an account" message instead, and the page shown is the same "check your email, then sign in" page in both cases. A password under 12 characters or over 1,024 bytes re-renders the form with a 422, the email kept; the length is checked before any hashing, so a long password never reaches argon2id (also at sign-in and password change).

**AC8. Sign in and refusal look alike.** Given an account, when its owner signs in with the right password, then a `__Host-session` cookie is set (`Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/`) whose value differs from any session cookie the browser sent, the previous session (if any) is ended on the server, and the account page shows their address; when the password is wrong, or the address has no account, then the response is a 422 with the same message in both cases and no cookie.

**AC9. Sign out ends the session on the server.** Given a signed-in session, when its owner signs out, then the cookie is cleared and a request replaying the old cookie is treated as signed out.

**AC10. Change password.** Given a signed-in account with a second signed-in session, when its owner changes the password with the correct current password, then the old password is refused and the new accepted, the second session is ended, and the dev outbox holds a "password changed" message; a wrong current password is a 422 and changes nothing.

**AC11. Delete account.** Given an account with sessions, reports, a problem report, a pending spot and an approved spot, when its owner confirms deletion with their password, then the user row, its sessions, reports, problem reports and pending spot are gone from the database, the approved spot remains with no author, and the visitor is signed out.

**AC12. Sign-in, sign-up and password change are rate limited.** Given five failed sign-in attempts for one address, or ten from one IP, within fifteen minutes, when a sixth (or eleventh) arrives, then the response is a 429 problem with `Retry-After`, even with the right password; sign-up and password change have their own limits.

### Contributing

**AC13. Contributing needs an account.** Given a signed-out visitor, when they open the add-spot form or a report form, then they are redirected to sign in with a return path that is one of our own paths; a `next` pointing off-site is replaced by `/`. A signed-out `POST` to either is a 401 problem.

**AC14. A new spot is pending.** Given a signed-in member, when they add a spot with a name, kind, area, address and declared attributes, then it exists with status `pending`, they see it marked as awaiting approval, and it is absent from search and from other visitors.

**AC15. Validation keeps what was typed.** Given a signed-in member, when they submit a spot with a blank name or an area over 80 characters, then the form re-renders with a 422, each problem named beside its field and in a summary, and the other values kept.

**AC16. A report shows at once.** Given a signed-in member and an approved spot, when they rate each attribute and add a note of up to 500 characters, then the report appears on the spot page with today's date and the spot's attributes reflect it; a note over 500 characters is a 422.

**AC17. Cross-site writes are refused.** Given a signed-in session, when any state-changing form is posted without its CSRF token, with another session's token, or with a cross-site `Origin` or `Sec-Fetch-Site`, then the response is a 403 problem and nothing changed; a `GET` to any write path is a 405 problem. Given a signed-out visitor, when the sign-in or sign-up form is posted without the pre-session token issued with the form, or with a cross-site `Origin`, then the response is a 403 problem and no session is created (login CSRF).

**AC22. Report a problem.** Given a signed-in member and a spot or report, when they report a problem with a reason, then it appears in the moderation queue for moderators and the member sees a confirmation.

### Moderation and roles

**AC18. Moderation is for moderators.** Given a member and a signed-out visitor, when either opens `/moderation` or posts to any moderation action, then the response is a 403 (member) or 401 (signed out) problem.

**AC19. Approve or reject.** Given a pending spot, when a moderator approves it, then it appears in search; when a moderator rejects it, then it is absent from search and its author sees it marked rejected.

**AC20. Hide a report.** Given a visible report, when a moderator hides it with a reason, then it is gone from the spot page, the spot's attributes no longer count it, and the moderation queue shows it hidden.

**AC21. Only an admin changes roles.** Given a member, when an admin sets their role to `moderator`, then they can open `/moderation`; when a moderator tries to change any role, then the response is a 403 problem and the role is unchanged.

### Cross-cutting

**AC23. Every error is a problem.** Given any request, when it hits an unknown path, a wrong method, an unknown resource or an unexpected failure, then the response is a problem with `type`, `title`, `status`, `detail`, `instance` and `correlationId`, as JSON for `Accept: application/json` and as an HTML page with the same fields for a browser; a 500 reveals nothing but the correlation ID.

**AC24. Security headers on every response.** Given any response, then it carries `Content-Security-Policy` without `unsafe-inline` or `unsafe-eval` (`default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'`), `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, and in production mode `Strict-Transport-Security`.

**AC25. One correlation ID per request, everywhere.** Given a request that queries the database and starts a goroutine (the welcome mail), when it completes, then every log line it produced, including the goroutine's, carries the same correlation ID, which is also in the `X-Correlation-Id` response header; an incoming `traceparent` is honoured.

**AC26. Nothing personal in the log.** Given a sign-up, a failed sign-in, a report with a note and a problem report, when the log output is searched, then it contains no email address, password, session token, note text or report text.

**AC27. Production refuses development values.** Given `WORKSPOTS_ENV=production`, when `DATABASE_URL` is missing, carries a development password, or `BASE_URL` is not `https`, then the process exits non-zero naming the variable (not its value); given development mode, the same configuration starts.

**AC28. Accessible in every state.** Given every page the end-to-end tests reach (empty, with results, validation failure, signed in and out, moderation), when axe-core runs against WCAG 2.2 AA, then it reports no violations; every journey can be completed with the keyboard alone with a visible focus indicator; at 320 CSS pixels wide (400% zoom) no page scrolls horizontally.

**AC29. Every journey works without JavaScript.** Given a browser with JavaScript disabled, when the end-to-end suite runs the search, sign-up, add-spot, report, moderation and deletion journeys, then every one passes.

**AC30. The app role owns nothing and has only what it needs.** Given an empty database with every migration applied, when the catalogue is queried, then `workspots_app` owns no object, has no `CREATE` on any schema, has `SELECT/INSERT/UPDATE/DELETE` only on the application tables and `USAGE` on their sequences, can execute no `SECURITY DEFINER` function, and `PUBLIC` has no privilege on the `public` schema.

## Release rehearsal

CI builds the production image, starts it against a fresh Postgres with production-style configuration (`WORKSPOTS_ENV=production`, a generated database password, `BASE_URL=https://localhost:8443`, `TRUSTED_PROXY` set to the proxy's address), runs the migrations, runs a smoke subset of the end-to-end tests, then scans the image. HTTPS comes from a TLS-terminating reverse proxy in front of the app (Caddy, pinned by digest, with its internal throwaway certificate authority), so the app's own configuration is exactly what production would get and AC27 is not weakened; the browser in the smoke tests trusts the throwaway certificate for that run only. The app itself never terminates TLS: that is the proxy's job in production too, and the app reads the client IP from `X-Forwarded-For` only when the connection comes from `TRUSTED_PROXY`.

## Slices

1. **Walking skeleton and gates:** server, config, correlation IDs, problem details, security headers, migrations, the app role, CI, hooks, `CLAUDE.md`, and the home page. AC1, AC23, AC24, AC25, AC27, AC30. After its first full CI run, branch protection goes on `main` (ADR 0010).
2. **Browse and search:** AC2–AC6, with the Playwright, axe, no-JavaScript and 320-pixel harness (AC28, AC29 from here on).
3. **Accounts:** AC7–AC10, AC12, AC17, AC26. Whole-codebase review after this slice.
4. **Contribute:** AC13–AC16, AC22.
5. **Moderation and roles:** AC18–AC21, with a `grant-role` command to bootstrap the first admin.
6. **Deletion, container and release rehearsal:** AC11, the Dockerfile, the release rehearsal, the security audit and the final review.

## Test tiers

Tier 1: Go acceptance tests drive the real server in-process over HTTP against a real Postgres, one database per test created and migrated from empty, dropped afterwards (ADR 0005). Tier 2: Playwright drives the built binary for the main journeys, axe and keyboard checks, with JavaScript off; the release rehearsal runs a smoke subset against the production image. Tier 3: none; there is no device surface.

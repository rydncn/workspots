---
paths:
  - "internal/web/**"
  - "internal/spots/**"
  - "internal/accounts/**"
  - "internal/moderation/**"
  - "cmd/**"
---

# Server conventions

Adapted from the playbook's `templates/rules/api.md` for a Go server that renders HTML.

## Secure by default

- Every route that changes anything needs a signed-in session, and the domain checks the specific resource (author, moderator, admin), not just "signed in". Every such route has a test where a different signed-in user is refused. Public routes are: `GET /`, `GET /search`, `GET /spots/{id}`, `GET /static/`, the sign-in and sign-up pages, and `GET /healthz`.
- Validate every form field and path value at the edge, in the handler, before the domain sees it; the domain validates again with its own named errors. Unknown fields are ignored (HTML forms), unknown paths are 404.
- A `GET` never changes state.

## Errors are RFC 9457 problems

- Every error response, including the mux's own 404 and 405 and validation failures, is a `Problem` from `internal/web/problems.go`: `type` (a stable path such as `/problems/spot-not-found`), `title` (fixed per type), `status`, `detail` (this occurrence), `instance` (the request path) and `correlationId`. A request whose `Accept` prefers `text/html` gets the same fields rendered by `problem.html`; otherwise `application/problem+json`.
- Validation failures (422) add an `errors` list of field and message; the HTML form shows the same list in a summary and beside each field.
- A 500 says nothing but the generic detail and the correlation ID; the error goes to the log with that ID.

## Domain errors are named

- Domain packages return sentinel errors named for the domain (`ErrSpotNotFound`, `ErrPermissionDenied`, `ErrPasswordTooShort`) or typed validation errors, never `errors.New("something failed")` at the call site. Wrap with `%w` to add context.
- Domain code never imports `net/http`. A domain error that ends the request (not found, refused, limited) is mapped once in `domainProblems`, added there in the same change. A domain validation error (`ErrPasswordTooShort`, `ErrInvalidEmail`, a wrong current password) is a field error the handler puts back on the form at 422, or in the `errors` list of the validation problem for a client that doesn't want HTML.
- Catch an error only to add context, recover or map it at the edge.

## Pages served to a browser

- Security headers come from the one `securityHeaders` middleware: the CSP in `middleware.go` with no `unsafe-inline` or `unsafe-eval`, HSTS in production, `nosniff`, `strict-origin-when-cross-origin`. No inline scripts or styles anywhere, so the CSP never needs loosening.
- The session cookie is `__Host-session`: `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/`, no `Domain`. The session ID is 256 random bits, stored as a SHA-256 hash, replaced at sign-in and at any change of privilege, and deleted on the server at sign-out.
- CSRF: every `POST` carries a token that is an HMAC-SHA256 of the session ID (or `anonymous`) keyed by the secret in the `__Host-csrf` cookie, and a same-origin `Origin` or `Sec-Fetch-Site`. Both cookies rotate at sign-in. A missing or wrong token is a 403 problem.
- Sign-in, sign-up and password change are rate limited per account key and per client IP; the client IP is `X-Forwarded-For`'s last hop only when the connection comes from `TRUSTED_PROXY`. Failure messages never say whether an account exists.
- Passwords: 12 to 1,024 bytes, checked before hashing; argon2id via `golang.org/x/crypto/argon2` with the parameters in `internal/accounts` (ADR 0012).
- Output goes through `html/template`'s contextual escaping. Any `template.HTML`, `template.JS` or `template.URL` on user data is a review comment.
- A `next` or return path is used only if it is one of our own paths (starts with a single `/`, no `//`, no scheme); otherwise `/`.

## How these are checked

- `TestAC23` asserts problem responses for the router's 404 and 405 and for a 500; `TestAC24` the headers; `TestAC17` (slice 3) CSRF and cookie flags on real responses; each domain error's mapping has a test that hits its route.
- CI's "HTTP stays in web" step fails if any file outside `internal/web` or `cmd` imports `net/http`.

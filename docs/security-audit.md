# Security audit (slice 6, 2026-11-10)

Run 1 of the playbook's `/security-audit` skill, quick profile, on commit `80a0a8a` plus the AC11 and release-rehearsal commits. The full artefacts (coverage ledger, findings.json, agent briefs) lived in the agent's container; this file is the target-neutral report and the leads, copied verbatim. What was done about each lead is in `docs/trial-log.md`, slice 6.


**Profile:** quick (one hunter wave, one final critic, one fresh verifier per candidate). **Scope:** the whole repository at commit `80a0a8a3d2ad` on `claude/workspots-slice-6-setup-ns9wy2`, clean tree. **Budget:** 12 agent invocations, 10 spent (4 reconnaissance, 3 hunters, 1 final critic, 2 verifiers). **Execution:** source-and-local-only; this container has no OS-enforced sandbox, so no target code was run and every decisive runtime fact is recorded as needs validation. **Prior runs:** none. **Partial pass:** a quick profile is a pre-declared early stop, not complete coverage; the final critic proposed two further units, recorded as deferred below.

### Posture

No confirmed vulnerability. Fourteen coverage units were reviewed from source: every state-changing route sits behind the CSRF middleware with an HMAC token and a same-origin check; sessions are random, hashed, rotated at sign-in and ended at sign-out, password change (others), role change and deletion; every moderation and admin action is checked in the handler and again in the domain; visibility is defined once in `spots.visible` and the public views; SQL is parameterised throughout; output goes through `html/template` with no raw HTML anywhere; logs carry ids, never addresses; the image is a static binary on distroless nonroot from a pinned base, and CI runs with a read-only token and pinned inputs. Two leads about the in-memory rate limiter need a deployment or runtime fact to settle and are listed below; both have been fixed in the slice 6 pull request since the audit ran (see the project's trial log).

### Confirmed findings

None.

### Needs validation

| Lead | Repository trace | Blocker |
|---|---|---|
| Unset or mismatched TRUSTED_PROXY collapses every per-IP rate limit into one shared bucket, letting one unauthenticated client lock sign-in and sign-up for everyone | `internal/web/accounts.go` → `internal/web/accounts.go` | The production value of TRUSTED_PROXY and the address (including IPv4 versus IPv6 form) the production proxy connects from are deployment facts not in the repository. |
| Sign-in requests already refused by the per-IP limit still allocate a per-address limiter entry, so limiter memory and sweep time grow with request rate rather than with the IP quota | `internal/web/accounts.go` → `internal/web/ratelimit.go` | Magnitude is a runtime fact: bytes retained per key, sweep duration at 10^5-10^6 keys under the shared mutex, and the refused-request rate reachable through the TLS proxy were not measured; no OS-enforced sandbox was available to run the limiter locally. |

Details, local next steps and owner-observed checks are in `NEEDS-VALIDATION.md`. Neither lead carries a severity.

### Hardening notes and positive patterns

From the hunters, none a finding on its own:

- .github/workflows/ci.yml: the postgres:16 service image is a mutable tag; pin it by digest like the Caddy image (runner-side only; no release path).
- .github/workflows/ci.yml 'Agent configuration changes': pull-request file names are echoed raw into the step summary, where markdown in a path renders; pass them through a code span. Advisory step, read-only token.
- Dockerfile: set ENV GOTOOLCHAIN=local in the build stage so a go.mod toolchain bump above the digest-pinned base fails the build instead of downloading a toolchain at build time.
- .github/workflows/ci.yml Trivy step: ignore-unfixed: true means an unfixed HIGH/CRITICAL in the base never fails the gate; if intended, record it beside the .trivyignore rule.
- cmd/workspots/main.go grant-role: the fallback to DATABASE_URL is not a privilege gain, but requiring MIGRATE_DATABASE_URL would keep the bootstrap tied to the operator credential. migrate and grant-role bypass config.Load, so WORKSPOTS_ENV=production does not refuse a development password for them.
- internal/accounts/accounts.go ValidateEmail admits NUL and invalid UTF-8, which Postgres rejects, so such a sign-up or /admin/users post answers a 500 instead of a 422; reject them as readSearch does.
- internal/web/spots.go readSearch allows page up to 100000 on /search; cap it or use keyset pagination. Ordinary work on a small table.
- e2e/global-teardown.js interpolates the database name from .server.json into DROP DATABASE unquoted; same-principal, but quoting the identifier costs nothing.
- The reviewed HEAD carried the deliberate PLANTED BREAK disabling the rehearsal's seed load; reverted in the next commit.
- internal/web/accounts.go: the per-address sign-in limit (5 failures per 15 minutes, AC12) lets any unauthenticated client lock a known victim address out of sign-in indefinitely by repeating five wrong passwords every 15 minutes; consider scoping the address bucket by address+IP, exponential backoff, or notifying the owner.
- internal/accounts/accounts.go ValidateEmail admits NUL bytes and invalid UTF-8, which Postgres refuses in sign-up, sign-in and the admin role form, producing a 500 instead of a 422; reject them at validation.
- internal/web/spots.go and internal/spots/spots.go: an anonymous GET /search?page=100000 forces a deep OFFSET costing a full ordered scan of approved spots per request; lower the cap or use keyset pagination.
- internal/web/middleware.go: any client may choose the request's correlation id via traceparent, so log lines of unrelated requests can be made to share an id; generate a server id and log the inbound trace id as a separate field if needed.
- internal/web/moderation.go hideReport builds the field name from the raw path id before pathID validates it; validate first.
- internal/config/config.go: production mode accepts an empty TRUSTED_PROXY without a warning; log a startup notice or refuse.
- internal/mail/mail.go: when a real provider replaces the Logging sender, reject control characters in To and Subject beyond the current space/tab/CR/LF check.
- internal/moderation/moderation.go: moderator() and the mutating UPDATE in SetStatus/HideReport/Resolve run as two statements without a transaction, so a role downgrade committed between them lets one stale action through. Fold the check into the UPDATE with an EXISTS on users.role.
- internal/accounts/accounts.go Delete: the CTE's `id NOT IN (SELECT id FROM public_spots)` is a snapshot-evaluated subquery; a concurrent approval committing during the statement may be re-checked against the stale subquery and the author's newly approved spot deleted. Use the row's own column (`status <> 'approved'`), which EvalPlanQual re-evaluates on the updated row version. Same-principal effect only.
- migrations/0005_one_vote_per_member.sql spot_attributes.report_count counts every visible report, not distinct voters, so one member's twenty reports make the page say 'From 20 reports' while one vote counts. Count DISTINCT author_id or show both numbers.
- internal/moderation/moderation.go SetStatus has no created_by <> actor check, so a moderator can approve a spot they added themselves. Trusted role in the design; add the check if segregation of duties is wanted.
- internal/moderation/moderation.go Decisions and templates/moderation.html show other moderators' email addresses to every moderator. Intended by AC19; a display name would keep addresses off a shared screen.
- internal/web/spots.go readSearch caps page at 100000 (OFFSET up to about five million). Settled as no finding: ORDER BY name over the approved set sorts every row regardless of page, and the set is bounded by moderation.
- internal/web/moderation.go hideReport echoes the raw path id into a field name before pathID parses it; html/template escapes it everywhere. Tidiness only.
- cmd/workspots/main.go and internal/config/config.go: the dev outbox is selected solely by WORKSPOTS_ENV defaulting to development; a deployment that forgets WORKSPOTS_ENV=production with a strong password writes addresses to stderr. Consider documenting the variable as mandatory in the Dockerfile or refusing a non-loopback ADDR in development.

Positive patterns worth keeping: the limiter is check-and-count under one lock and refunded on success; `Authenticate` verifies a dummy hash for unknown addresses; `safeNext` parses rather than inspects; `.dockerignore` is an allowlist; the migrate role must own the database; every error is a problem with a fixed detail.

### Coverage

Ledger: 12 covered, 2 candidate (both independently verified as needs validation), 0 blocked, 2 deferred. Companions selected: web protocol and auth, data isolation and lifecycle, resource exhaustion, supply chain and release; client-side, cloud, protocols, AI, memory safety and desktop/IPC excluded because no such surface exists. The final critic did not return `stop: true`: it proposed (1) a unit on the migration runner and the database privilege model (who owns `schema_migrations`, default privileges granting DML on future views, the global function REVOKE, the ownership check with the app role) and (2) a unit on the mismatch between Go's `strings.ToLower` in the per-address limiter key and Postgres `lower(email)` as the account identity. Both are `deferred` with reason `quick_profile_final_critic` for a later run.

## Needs validation

Source-grounded leads whose decisive fact is outside the repository or needs a runtime measurement. No severity is assigned.

#### Unset or mismatched TRUSTED_PROXY collapses every per-IP rate limit into one shared bucket, letting one unauthenticated client lock sign-in and sign-up for everyone

**Fingerprint:** `web.clientIP.untrusted-peer-shared-ip-bucket` · **verdict:** needs_validation (no severity)

**Repository trace.** `internal/web/accounts.go:96` (signInPost) → `internal/web/session.go:167` (clientIP) → `internal/web/session.go:173` (clientIP) → `internal/config/config.go:41` (Load) → `internal/web/accounts.go:97` (signInPost)

**What the source shows.** clientIP (internal/web/session.go:162-174) returns the TCP peer host unless cfg.TrustedProxy is non-empty and equal to that host, in which case it returns the last X-Forwarded-For hop. config.Load (internal/config/config.go:41) reads TRUSTED_PROXY with getenv and the production checks (config.go:56-64) cover only the database password and BASE_URL, so an empty or stale TRUSTED_PROXY starts without error or log line; README.md:25 nevertheless states that production refuses a missing value, which the source does not do for this variable. The design mandates a TLS-terminating reverse proxy in front of the app (docs/design.md:153). Behind that proxy, with TRUSTED_PROXY unset or not equal to the proxy's connecting address, every request has the proxy's address as its peer, so the per-IP buckets are site-wide: signin:ip (signInPerIP = 10 per 15 min, accounts.go:96-97, counted before Authenticate and refunded only on success), signup:ip (signUpPerIP = 10, accounts.go:64), pwchange:ip (accounts.go:159-160) and delete:ip (accounts.go:202-203). Ten failed POST /sign-in requests from one unauthenticated client (any emails, wrong passwords, each with the anonymous CSRF token obtained from GET /sign-in) make every later POST /sign-in from every user a 429 with Retry-After for up to 15 minutes, repeatable indefinitely at ten requests per window; ten sign-up attempts do the same for sign-up. The repository's own test (internal/web/accounts_test.go:326-342) demonstrates exactly this shared bucket when TRUSTED_PROXY is unset, framed as the forged-header case. Whether production sets TRUSTED_PROXY to the address the proxy actually connects from, and whether the app port is reachable only through the proxy, are deployment facts not in the repository.

**Claimed root cause.** internal/web/session.go clientIP fails open to the peer address when TRUSTED_PROXY is empty or does not match the peer, and internal/config/config.go Load does not require or validate TRUSTED_PROXY in production mode (contrary to README.md:25), so a deployment behind the mandated proxy with a missing or stale value keys all clients on the proxy's single address with no error or log line.

**Blockers.**
- The production value of TRUSTED_PROXY and the address (including IPv4 versus IPv6 form) the production proxy connects from are deployment facts not in the repository.
- Whether the app's listener is reachable only through the proxy (and so whether every request shares one peer address) is not in the repository.
- No sandbox was available to run the binary behind a loopback proxy fixture and observe the site-wide 429; the equivalent observation exists only as the repository's own test at internal/web/accounts_test.go:326-342, which this audit did not execute.

**Bounded local next step.** Non-destructive, against a throwaway database. (1) Run go test ./internal/web -run 'AC12' and confirm the subtest "X-Forwarded-For is ignored without a trusted proxy" passes: it shows ten failed sign-ins from ten distinct forwarded addresses make an eleventh distinct client's attempt a 429 when TRUSTED_PROXY is unset. (2) Start the binary in production mode (WORKSPOTS_ENV=production, https BASE_URL, non-development database password) behind any loopback reverse proxy with TRUSTED_PROXY unset or set to an address other than the proxy's connecting address: observe it starts with no error or log line about TRUSTED_PROXY. From one client, GET /sign-in to obtain the __Host-csrf cookie and anonymous token, then send ten POST /sign-in with wrong passwords (any emails); then from another client send a correct sign-in for a known account: expect 429 with Retry-After. Repeat with TRUSTED_PROXY equal to the proxy's connecting address and distinct X-Forwarded-For values: expect the correct sign-in to succeed (303). (3) A unit test can assert that config.Load in production mode currently accepts an empty TRUSTED_PROXY.

**Owner-observed deployment check.** The operator compares the running container's TRUSTED_PROXY with the exact source address the proxy uses to connect to the app (as the app sees it in RemoteAddr, including IPv6 or container-network forms), confirms the app port is not reachable except via the proxy, and confirms that after ten failed sign-ins from one client address other clients can still sign in, rather than receiving 429 site-wide.

#### Sign-in requests already refused by the per-IP limit still allocate a per-address limiter entry, so limiter memory and sweep time grow with request rate rather than with the IP quota

**Fingerprint:** `web.signInPost.addr-take-before-ip-limit` · **verdict:** needs_validation (no severity)

**Repository trace.** `internal/web/accounts.go:88` (signInPost) → `internal/web/accounts.go:97` (signInPost) → `internal/web/ratelimit.go:37` (limiter.take) → `internal/web/ratelimit.go:40` (limiter.take)

**What the source shows.** signInPost (internal/web/accounts.go:97) evaluates max(s.limits.take(addrKey, 5), s.limits.take(ipKey, 10)). Go evaluates both arguments, and the address key is taken first, so once an unauthenticated client's IP bucket holds ten failures every further POST /sign-in from that IP is refused with a 429 yet still inserts one new hashed key (a 32-byte map key, a slice header and a one-element []time.Time) when the posted email is one the limiter has never seen. The email is only required to be non-empty, so its cardinality is attacker-chosen. The only pre-request costs are one GET /sign-in to obtain an anonymous CSRF cookie and token, which is reusable, and an Origin header; no database query or hash runs on the refused path, so the attacker's per-request work is a cheap HTTP POST. Keys are removed only by prune, which runs for a key when that key is next consulted (never, for a random address) or in the sweep at every 1000th accepted take across all rate-limited handlers; the sweep iterates every key under the single limiter mutex that sign-in, sign-up, password change, deletion and contribution handlers all take. Steady-state size is therefore bounded by request rate times the 15-minute window, not by the IP quota, and the amortised sweep cost per accepted take grows linearly with the key count. After the burst stops, the keys persist until 1000 further accepted takes occur, so on a quiet site they outlive the window by a wide margin. The repository's own CI proxy is Caddy's plain reverse-proxy with no rate limit (.github/workflows/ci.yml:230); whether the production proxy caps per-client request rate is not in the repository. Magnitude (bytes per key, sweep duration at 10^5-10^6 keys, achievable refused-request rate through the proxy) cannot be settled from source and no sandbox was available, so this remains a candidate with a plausible but unmeasured shared effect.

**Claimed root cause.** internal/web/accounts.go:97 takes the per-address key before and independently of the per-IP key, and limiter.take (internal/web/ratelimit.go:31-48) counts and inserts a new key whenever that key alone is under its limit, before the caller combines the two results, so requests the IP gate refuses still grow limiter state; the limiter has no aggregate key cap and its sweep is O(keys) under the shared mutex.

**Blockers.**
- Magnitude is a runtime fact: bytes retained per key, sweep duration at 10^5-10^6 keys under the shared mutex, and the refused-request rate reachable through the TLS proxy were not measured; no OS-enforced sandbox was available to run the limiter locally.
- Whether the deployed reverse proxy (production is behind a TLS proxy; the repository's Caddy invocation has no rate limit) enforces a per-client request-rate cap on POST /sign-in is not in the repository.
- Whether Go map memory is returned after keys are deleted (peak size may persist for the process lifetime) is a runtime property not settled from source.

**Bounded local next step.** In package web add a test: l := newLimiter(15*time.Minute); call l.take("signin:ip:x", 10) ten times; then for i in 0..9999 evaluate wait := max(l.take(fmt.Sprintf("signin:addr:%d@x", i), 5), l.take("signin:ip:x", 10)) and assert wait > 0 for every iteration (all refused) yet l.size() == 10001 afterwards, proving refused requests grow state; additionally assert that after a further 999 accepted takes on other keys the 10000 address keys are still present (no sweep yet) to show retention is tied to accepted-take count, not time. Then a benchmark that pre-fills 10^6 distinct keys and times a single accepted take that lands on records%1000 == 0 to measure sweep duration under the mutex, and report runtime.MemStats HeapInuse before and after filling to measure bytes per key. Run with go test -run TestLimiter -bench Sweep ./internal/web under a memory limit.

**Owner-observed deployment check.** The operator checks whether the proxy in front of the app enforces a per-client request-rate limit on POST /sign-in (and /sign-up), and in a staging copy sends a short, bounded burst (for example 20,000 POST /sign-in requests with distinct email values from one address after its IP bucket is full) while recording the app's resident memory and the latency of a concurrent member's sign-in; stop at the first measurable stall or memory step, and observe whether memory returns after 15 minutes plus 1000 further accepted requests.

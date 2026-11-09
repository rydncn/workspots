# Trial log: Workspots under the ai-playbook core

Trial 2 of the [ai-playbook](https://github.com/rydncn/ai-playbook) at commit `f2fd445`, applying only the core ("Start here") to a stack the playbook wasn't written for: Go, server-rendered HTML, Postgres, npm only for `e2e/`. Compared at the end with trial 1 (a small Expo app under the full playbook: stronger on all four review axes, 9 findings, two to three times the effort per behaviour).

How context is measured: the playbook's context-budget hook reads the transcript's last usage figure. In this cloud environment the harness's own system prompt and tool definitions are about 80k tokens before any work, so every figure below includes that floor.

## Per slice

### Slice 0: design and setup (this log, the design, ADRs, `scripts/dev-db.sh`)

- Criteria delivered: none (design only).
- Sessions: 1. Peak context: hook not yet installed; about 160k reported by the harness at the end, of which about 80k is the floor. Wall clock: about 1.5 hours including Ryan's review.
- Defects found: 4 by the human design review (login CSRF uncovered, session rotation missing from AC8, no maximum password length, rehearsal HTTPS versus AC27). None by tooling: nothing to test yet.

### Slice 1: walking skeleton and gates (PR #1)

- Criteria delivered: AC1, AC23, AC24, AC27, AC30; AC25 without the goroutine clause (no goroutine exists yet; slice 3 adds the first).
- Sessions: 1, the same session as the design, continued at Ryan's request past the budget. Peak context: the hook was not installed in this session; the harness showed roughly 300k tokens by the end, including the 80k floor. Wall clock: about 3 hours from the first line of code to the green full CI run, including two waits on Ryan (branch protection, the throwaway branch).
- Defects by layer:
  - tests (test-first): 2. A catalogue query in the AC30 test resolved `has_table_privilege` against the wrong schema (failed with "relation collations does not exist"); the catch-all `/` route turned `POST /` into a 404 instead of a 405.
  - race detector: 1. The test's log buffer was read without its lock; an intermittent failure under `-race` that the plain run never showed.
  - real database (rule 16): 1. `DROP DATABASE … WITH (FORCE)` failed intermittently with "permission denied to terminate process" because the migrate role could not end the app role's connections; fixed in `scripts/db-init.sql`, lesson in `CLAUDE.md`.
  - hooks: 0 defects in the code; both hooks were proven by planted errors (vet error → after-edit exit 2; unformatted file and failing test → before-stop block; `stop_hook_active` → no loop).
  - fresh-context review (subagent, diff + design + `CLAUDE.md`, about 140k tokens): 10 findings, 4 must-fix, all verified and all fixed test-first.
    1. MUST `ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC` stored nothing (a per-schema default ACL starts empty), so a later SECURITY DEFINER function would have been executable by the app role; the AC30 clause was vacuous. Now global; the test creates such a function and checks.
    2. MUST The CI grep for `context.Background()` caught `internal/testdb`, so the checks job was red on the branch as committed; the rule text and the check now agree.
    3. MUST `go test | tee` without pipefail: the Tests job would pass on a failing test (CI confirmed it on #2's first run). `shell: bash` plus a `FAIL` grep.
    4. MUST Static files answered a missing file with the file server's plain-text 404 and `/static/` with a directory listing; now problems, with tests.
    5. SHOULD `REVOKE ALL ON SCHEMA public FROM PUBLIC` by a non-owner is only a warning; the first migration now refuses a database the migrate role doesn't own, with a test.
    6. SHOULD HSTS in production was never exercised; AC24's test now runs a production-configured handler.
    7. SHOULD The 405 problem had no `detail`; added, and every problem is now checked for type, title, detail and correlation ID.
    8. SHOULD `main` was untested for AC27; `run()` now has a test.
    9. SHOULD AC1's test didn't assert name order; it does.
    10. SHOULD `recover` swallowed `http.ErrAbortHandler`; govulncheck was `@latest` while everything else was pinned. Both fixed.
  - found while fixing a review finding: 1. The route wrapper called the matched handler directly rather than through the mux, so wildcard path values were never bound; invisible until the static route used `{file...}`.
  - CI, first runs: 2 defects seen, both already named by the review before CI confirmed them: the `testdb` grep (run 2 on PR #1, checks job red) and the Tests job passing a planted failing test (run 3, on #2).
  - CI, process: 1. The planted token on the throwaway branch failed the secrets scan on PR #1 too (checkout fetches every branch and gitleaks scans every ref); the branch could not be deleted from this session, so Ryan deleted it.
  - own proof invalidated: 1. The first local gitleaks "proof" grepped for "leaks found", which also matches "no leaks found", and the planted token was 36 of one letter, under gitleaks' entropy threshold. Re-proved with a random token, locally and in CI.
  - own process: 1. This slice 1 entry was first written while the reviewer ran, then swept into the throwaway branch's commit by `git add -A`, so it was missing from PR #1 until Ryan noticed; two later edits to it silently matched nothing. Restored here.
- Gates broken on purpose, by layer (each seen to fail once):
  - locally: gofmt, go vet, staticcheck, go test, TestAC30 (grant commented out), the two rule greps, the "Tests ran" check, zizmor (unpinned action), gitleaks (random token), all three hooks.
  - in CI (throwaway PR #2, second run): gofmt failed the checks job; the planted test failed the Tests job; the random token failed the security job. Not provable locally: govulncheck (database unreachable through the proxy; green in CI).
- Branch protection on `main` after the first full run (ADR 0010): ruleset active, no bypass, pull request with code owner review, the three jobs required by name; recorded in a comment on PR #1.

### Slice 2: browse and search (PR #4)

- Criteria delivered: AC2, AC3, AC4, AC5, AC6; the AC28 and AC29 harness (axe, 320 pixels, keyboard, JavaScript off) over every page and state in this slice.
- Sessions: 1, still the first session, continued at Ryan's request; the harness showed roughly 380k tokens by the end of the slice. Wall clock: about 1.5 hours to the draft PR.
- Defects by layer:
  - tests (test-first): 1. The AC30 "at least one application table" check was written as exactly one, so migration 0002 failed it; corrected to a minimum.
  - end-to-end: 2. axe-core cannot run in a page with JavaScript disabled (the whole suite timed out on it); axe now runs on the page's HTML in a separate JavaScript-enabled context, which also covers POST result states later. A strict-mode locator matched two elements.
  - gate breaks: 1 invalid proof. The first attempt to break the 320-pixel check used an inline `style` attribute, which the CSP blocks, so nothing was wide and the check "passed"; re-broken with a 300-character unbroken word, which it caught.
  - own process: 1. `git checkout` of a template to undo a planted break reverted the slice's uncommitted changes to it as well; rewritten from the session. Lesson: commit before planting breaks, or plant them in a copy.
  - hooks: 0.
  - CI, draft run: 0 (the new npm lockfile, signature and audit steps passed first time).
  - CI, first full run: 1. The e2e job failed at the template's npm version check: Node 22 on the runner ships npm 10.9, below the 11.10 that `min-release-age` needs. The security job had run `npm ci` with the same npm, so its release-age setting had silently not applied. Both jobs now install npm 11 first.
  - fresh-context review (subagent, about 138k tokens): 10 findings, 2 must-fix, all verified and fixed test-first.
    1. MUST A NUL byte in the `area` query reached Postgres and became a 500; the 80-character cut was by byte, so a multibyte character could be split. Now cleaned at the edge, with a test.
    2. MUST The e2e setup polled the port without checking its own child was alive, so the suite could pass against a server it did not start. It now refuses an occupied port (proven by starting another server first) and fails if the child exits.
    3. SHOULD The axe helper could audit an unstyled or empty document; it now asserts the stylesheet applied and that axe recorded passes.
    4. SHOULD The keyboard test proved two focus stops; it now walks every control to the button, submits, tabs to a result and opens it, requiring the site's own 3px ring rather than the browser default.
    5. SHOULD A failed setup orphaned its database; the state file is now written right after `CREATE DATABASE`.
    6. SHOULD The area as displayed ("Leeds, LS6") found nothing; every typed word must now match, with a test.
    7. SHOULD The view had write grants from the default privileges and ran with owner rights; now `security_invoker` and read-only, and `TestAC30` checks both for every view.
    8. SHOULD Median and majority ties were accidental; now a rounded midpoint and a strict majority, commented.
    9. SHOULD "Quiet" (noise) beside "Quiet space" was ambiguous to a screen reader, and the AC5 assertion on it near-tautological; now "Noise: Quiet", with a sharper assertion.
    10. SHOULD Out-of-range filter values were applied but echoed as "Any"; the form now shows the clamped value, with a test. Leftover template text in `.npmrc` removed.
- Gates added and broken once: axe (image without alt → `image-alt`), the 320-pixel horizontal-scroll check (unbroken word), the end-to-end assertions (locator failure seen). lockfile-lint ran clean locally. Not provable locally: `npm audit signatures` (the registry's attestation download is blocked by the proxy); proven in CI.

### Slice 3: accounts (PR #5)

- Criteria delivered: AC7, AC8, AC9, AC10, AC12, AC17, AC26, and the goroutine clause of AC25. Whole-codebase review follows this slice.
- Sessions: 1, still the first session, continued at Ryan's request; the harness showed roughly 470k tokens by the end of the slice. Wall clock: about 2 hours to the draft PR.
- Defects by layer:
  - tests (test-first): 4. A Go template can't range over an untyped nil, so every form page was a 500 until `Errors` defaulted to a typed nil slice; the form-errors partial wasn't in the parse list; the CSRF middleware ran before route matching and turned a `POST /` into a 403 instead of the 405 AC23 requires; and the test browsers all shared one cookie jar (`httptest.Server.Client()` returns one client), which made two tests pass for the wrong reason and two fail mysteriously until each browser got its own client.
  - tests (unit): 0 beyond the above; the argon2id round trip and the length rules passed first time.
  - end-to-end: 2. The nav's new links changed the keyboard tab order the browse journey asserted; a "Sign up" locator matched the nav and the page body.
  - gate breaks: CSRF comparison disabled → AC17's token-based refusals fail (the first attempt at this break didn't compile, so proved nothing; redone); sign-in limit raised to 500 → AC12's "sixth attempt" fails; other sessions no longer ended → AC10 fails.
  - hooks: 0.
  - CI, draft run: 0.
  - fresh-context review (subagent, about 142k tokens): 8 findings, none must-fix, all fixed with a test where one applies and seen failing first.
    1. Rate-limit keys were raw addresses and IPs held for the process lifetime; now hashed and dropped when their window empties (unit test).
    2. Expired sessions were filtered, never deleted, so the 30-day retention was a read-time rule; now deleted at the next sign-in (AC8 test extended).
    3. The validation problem's `errors` list had `Field`/`Message` keys, not the rule's `field`/`message`; JSON tags added.
    4. The JSON-validation assertion accepted 403 or 422 and could not fail; rewritten to fetch the token first and assert the list.
    5. Two simultaneous sign-ups for one address raced to the unique index and the loser got a 500; SQLSTATE 23505 is now "already registered".
    6. The mail goroutine had no recover; a panicking provider would have taken the process down.
    7. Rule contradictions: validation errors were mapped on the form rather than in `domainProblems` (rule text amended to say so); password change was limited per account only, now per IP too; `__Host-csrf` flags were asserted nowhere, now in AC8; the current password is checked before the new one is validated.
    8. The untrusted-proxy branch of `clientIP` was never observed; a test now shows a forged `X-Forwarded-For` cannot escape the per-IP limit.
  - whole-codebase review (subagent briefed from `CLAUDE.md`, the design, the rules and the ADRs, no diff; about 205k tokens): 12 findings, 4 must-fix, all fixed, the behavioural ones test-first.
    1. MUST `/static/` responses were `Cache-Control: public` and carried the `__Host-csrf` Set-Cookie, so a shared cache could hand one CSRF secret to every browser: login CSRF, the case AC17 exists for. Static and health responses now skip the session middleware; a test asserts no Set-Cookie.
    2. MUST The HTML problem page omitted `type`, contradicting AC23 and `api.md`, and the test didn't check it. Rendered and asserted.
    3. MUST The design and `CLAUDE.md` said every `POST` must carry a same-origin `Origin` or `Sec-Fetch-Site`; the code accepted a `POST` with neither. Now refused, with a test; the design's "same-site" wording corrected to "same-origin".
    4. MUST `GET /healthz` was documented in `api.md` and ADR 0008 and did not exist. Added: 200 "ok", 503 problem when the database is down, `no-store`; tested both ways.
    5. The e2e suite would hit the in-memory sign-up limit as soon as slice 4 added a journey; users are now seeded per project and only the sign-up journey signs up.
    6. AC12's password-change limit and AC6's author/moderator exception were named but not exercised; the first is now tested, the second recorded in the design as owed to slices 4 and 5.
    7. `TestApplyIsIdempotentAndOrdered` never applied twice; it does now (and fails when `ON CONFLICT DO NOTHING` is removed).
    8. The design claimed a faked clock that doesn't exist; wording corrected.
    9. `CLAUDE.md`'s architecture omitted `internal/mail` and the `sessions`/`csrf` middleware; updated.
    10. `net/http/httptest` was imported by the production binary for the 405 probe; replaced with a ten-line header-capturing writer.
    11. Retention table versus code: expired sessions are now also deleted at sign-out and indexed by expiry, and the table says when deletion happens and that rate-limit keys are pruned on use.
    12. Copy: the check-email page told existing account holders to use "the password you chose"; the CODEOWNERS header still said "copy to". Both fixed.
  - Per-slice reviews had passed all twelve; four only show when the slices are read together (static caching versus the new cookie, the document-versus-code Origin rule, the documented health route, the e2e harness versus the new limits).
  - outside review by Ryan (after both automated reviews): 3 defects missed by the tests, the fresh-context review and the whole-codebase review, plus 2 design points. All fixed test-first.
    1. MISSED Open redirect: `safeNext` allowed a tab, and browsers strip tabs from URLs, so `/sign-in?next=/%09/evil.example` sent the visitor off-site after sign-in (proven with a Go server and Chromium). Now parsed: no scheme, host or user, a single leading slash, and every control character and backslash refused; twelve-case unit test plus a real sign-in with the tab.
    2. MISSED Rate-limit race: the limit was checked before `Authenticate` and counted after it failed, so concurrent attempts all passed. Now one atomic check-and-count before the hash, refunded on success, for sign-in, sign-up and password change; a test fires twenty wrong passwords at once and sees exactly five checked and fifteen refused.
    3. MISSED Concurrent argon2id was uncapped (19 MiB each). Hashing now runs behind a semaphore of `runtime.NumCPU()` slots; a request waits for a slot until its context ends, recorded in ADR 0012; a unit test shows one slot serialises eight hashes and a cancelled context does not wait.
    4. Spot visibility was decided in two queries (`status = 'approved'` in both `Search` and `Get`), the pattern that diverges when slices 4 and 5 add the author and moderator exceptions; now one `visible` constant that both queries use.
    5. Search had no limit; now fifty a page with next and previous links that keep the search, tested with fifty-one spots.
  - Why the automated layers missed the first three: both reviewers read `safeNext` as a path check and did not think about what browsers do to a URL before following it; both saw the limiter and reasoned about one request at a time; neither asked what a burst of sign-ins costs in memory. All three are concurrency or browser-behaviour questions, which neither brief asked for by name.
  - own process: 2. A stacked pull request (#5 on #4's branch) was merged after its base had merged into `main`, so GitHub merged it into the stale branch and `main` missed slice 3; a third pull request (#8) carries the same commits over. Stacking cost a round trip; next time wait for the base to merge, or retarget before merging. A staticcheck failure in a chained shell command stopped the documentation edits but not the commit that followed, so one commit went out with a lint failure and without its records; fixed in the next.

### Slice 4: contribute (PR #9)

- Criteria delivered: AC13, AC14, AC15, AC16, AC22, and AC6's author exception (a pending spot is visible to its author, as "awaiting approval").
- Sessions: 1, still the first session, continued at Ryan's request after slices 2 and 3; the harness showed roughly 560k tokens by the end of the slice. Wall clock: about 1.5 hours to the draft PR.
- Defects by layer:
  - tests (test-first): 0 in the implementation; all five acceptance tests passed on the first run after the handlers existed (each had failed on the missing routes). The visibility rule was already in one place (`spots.visible`, Ryan's slice 3 point), so adding the author took one line and no query diverged.
  - gate breaks: author dropped from `visible` → AC14 "author's view: 404"; sign-in requirement removed from the add-spot form → AC13 "GET /spots/new: 200". Raising the handler's note limit did **not** fail AC16: the database's `length(note) <= 500` constraint refused the row and the handler mapped it to the same 422, so the handler check and the constraint back each other up and the test cannot tell which one fired. Recorded, not changed.
  - end-to-end: 3. The browser's own `required` attribute stopped invalid forms before the server saw them, so the server's validation state could not be reached (the test now removes the attribute first, as the accounts test already did for `minlength`); Tab reaches a radio group at its checked member, not its first; and the report journeys added reports to a seeded spot the browse journeys assert attributes on, which turned those journeys red under parallel runs: the report journeys now have their own seeded spot.
  - hooks: 0. CI, draft run: 0.
  - fresh-context review (subagent, about 141k tokens): 7 findings, none must-fix, all fixed, with a test seen failing first where one applies.
    1. The problem form wrapped any database error from the spot lookup in the 404 sentinel, so an outage on that path would have shown as "nothing to report" and gone unlogged; only a real not-found is mapped now.
    2. AC13's POST assertion accepted 401 or 403 and posted without a token, so two of the three paths were only checked by CSRF; now every path is posted with a valid pre-session token and must answer 401, and the dead off-site-next lines are gone (that case lives in the session tests).
    3. The error summary's links to the rating groups went nowhere: the fieldsets now carry `id` and, when invalid, `aria-describedby`.
    4. AC22 had no negative cases; a hidden report and a pending spot (by its author) now get 404 on both the form and the post, with no row.
    5. AC26 never posted a note or a reason; it does now, including a refused over-long note, and none reach the log.
    6. Contributions were unlimited; now 10 spots, 20 reports and 20 problem reports per member per quarter hour through the existing limiter, tested.
    7. An index on `spots.created_by`; the problem form for a report checks the report is visible on GET as well as POST; the report form for one's own pending spot is 404 like the post.
  - gate breaks for the review's tests: sign-in check dropped from the report POST → the strengthened AC13 fails (the handler then wrote a 401 problem and carried on to a 500, so the body was two problems; the old assertion would have passed on the 403 CSRF gave a token-less post); hidden-report checks dropped → AC22 "problem form for a hidden report: 200" and "problem on a hidden report: 200".

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

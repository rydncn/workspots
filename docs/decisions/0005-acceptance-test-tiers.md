# 0005. Acceptance tests at tier 1 in Go and tier 2 with Playwright

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

Rule 16 asks for the tier to be recorded. Tests must be isolated, parallel and leave nothing behind.

## Decision

Tier 1: Go tests start the real server in-process with `httptest` against a real Postgres, each test with its own database, created and migrated from empty and dropped afterwards. Tier 2: Playwright drives the built binary with JavaScript off for the main journeys, axe and keyboard checks; a smoke subset runs against the production image in the release rehearsal. Tier 3: none, there is no device surface.

## Rejected options

- **One shared test database with truncation between tests:** serialises the suite and breaks under `t.Parallel()`.
- **A transaction per test:** the server has its own pool, so the test cannot share a transaction with it.
- **A schema per test:** needs per-connection `search_path` and per-schema grants, which would make the privilege test test something production never runs.

## Consequences

Creating and migrating a database costs about a hundred milliseconds per test, acceptable for tens of tests. The development migrate role needs CREATEDB; the production one does not.

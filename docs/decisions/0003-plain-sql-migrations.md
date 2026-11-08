# 0003. Plain SQL migrations applied by our own runner

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

Migrations must run on an empty database in CI on every change, in the release rehearsal and before tests.

## Decision

Numbered `migrations/NNNN_name.sql` files applied in order inside a transaction each, recorded in `schema_migrations`, by `internal/migrate` (about sixty lines) exposed as `workspots migrate`.

## Rejected options

- **golang-migrate, goose, tern:** each is a dependency and a binary to pin and audit for a job that is a sorted directory listing and a version table. Down migrations are not needed: a mistake is fixed forward.
- **Schema-diff tools (atlas, sqldef):** the diff is a one-way door we want to read as SQL.

## Consequences

No down migrations. Revisit if migrations need to run while an old version still serves traffic.

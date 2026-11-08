# 0002. Use pgx for Postgres

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

The standard library has no Postgres driver.

## Decision

`github.com/jackc/pgx/v5` with `pgxpool`, used directly rather than through `database/sql`, with hand-written SQL.

## Rejected options

- **`database/sql` with `lib/pq`:** `lib/pq` is in maintenance mode and its own README recommends pgx.
- **GORM or another ORM:** hides the SQL the privilege tests and migrations must agree with.
- **sqlc:** a code generator and build step for a schema of five tables; worth it at a larger size.

## Consequences

Queries are plain SQL in the domain packages. Revisit if the query count makes sqlc's checked SQL worth its generator.

# 0004. Two database roles: a migrator that owns the schema and an app role that owns nothing

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

Rule 2 of the playbook: privileges are verified in the catalogue, not inferred from migration files. A role that owns a table can do anything to it.

## Decision

`workspots_migrate` owns every object and runs migrations. `workspots_app` holds only CONNECT, USAGE on the schema, SELECT/INSERT/UPDATE/DELETE on application tables and USAGE on their sequences, granted by default privileges set in the first migration so a new table cannot be forgotten. `PUBLIC` has nothing on the `public` schema. A test (AC30) checks this on an empty migrated database.

## Rejected options

- **One role for everything:** simplest, and the first SQL injection owns the schema.
- **Row-level security:** the app has no multi-tenant rows; authorisation lives in the domain.

## Consequences

A table that needs fewer grants (an audit log the app only inserts into, say) revokes them in its own migration; AC30's test is where that would be noticed.

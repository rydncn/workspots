# 0011. Account deletion removes everything personal; approved spots stay, anonymised

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

Deletion must remove what the account owns, proved by a test that looks in the database.

## Decision

Deleting an account removes the user, its sessions, its reports, its problem reports and any pending or rejected spot it added. An approved spot describes a public place, not the person: it stays with `created_by` set to null.

## Rejected options

- **Delete approved spots too:** destroys community content that holds no personal data.
- **Soft-delete the user:** keeps the email and hash around, which is what deletion exists to remove.

## Consequences

A deleted author's reports vanish from spot pages, so spot attributes recompute. Revisit if attribution of spots becomes a feature.

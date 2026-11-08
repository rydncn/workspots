# 0010. A person merges on green CI; the agent asks

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

No branch protection is assumed on this repository. Rule 9: when nothing blocks a red merge mechanically, write down who merges and on what evidence.

## Decision

Only Ryan merges, on a pull request out of draft with every CI job green. The agent opens draft pull requests and never pushes to `main` or merges without asking; `.claude/settings.json` sets those commands to ask. CI on `main` is the backstop and is never cancelled.

## Rejected options

- **Agent merges on green:** removes the one human read of every one-way door.

## Consequences

Merges wait for a person. Turn on branch protection with required checks if the plan offers it.

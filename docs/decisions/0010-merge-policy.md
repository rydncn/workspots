# 0010. A person merges on green CI; the agent asks

**Status:** accepted · **Date:** 2026-11-08 (amended the same day: the repository is public, so branch protection is available) · **Replaces:** none

## Context

The repository is public, so GitHub's branch protection is available. Rule 9: CI only protects anything while it is required, and until protection is on, nothing blocks a red merge mechanically.

## Decision

Only Ryan merges, on a pull request out of draft with every CI job green. The agent opens draft pull requests and never pushes to `main` or merges without asking; `.claude/settings.json` sets those commands to ask. CI on `main` is the backstop and is never cancelled.

Branch protection goes on `main` as soon as slice 1's pull request has had its first full CI run, so the check names exist to require: pull requests only, every CI job required by name, code owner review required, no force pushes, no deletions, and no bypass for administrators. The agent stops at that point and asks Ryan to turn it on, then reads the protection back from the GitHub API and records what it found in the pull request. Until then, the first slice merges on the same evidence by hand.

## Rejected options

- **Agent merges on green:** removes the one human read of every one-way door.

## Consequences

Merges wait for a person, and once protection is on, for green required checks and a code owner's review, with no administrator bypass. Renaming a CI job means updating the required checks in the same change.

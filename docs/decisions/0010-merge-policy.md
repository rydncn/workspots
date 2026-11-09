# 0010. A person merges on green CI; the agent asks

**Status:** accepted · **Date:** 2026-11-08 (amended the same day: the repository is public, so branch protection is available) · **Replaces:** none

## Context

The repository is public, so GitHub's branch protection is available. Rule 9: CI only protects anything while it is required, and until protection is on, nothing blocks a red merge mechanically.

## Decision

Only Ryan merges, on a pull request out of draft with every CI job green. The agent opens draft pull requests and never pushes to `main` or merges without asking; `.claude/settings.json` sets those commands to ask. CI on `main` is the backstop and is never cancelled.

Branch protection goes on `main` as soon as slice 1's pull request has had its first full CI run, so the check names exist to require: pull requests only, every CI job required by name, no force pushes, no deletions, and no bypass for administrators. Code owner review is **not** required: every pull request is authored by Ryan's account (the agent commits as Ryan), GitHub doesn't let an author approve their own pull request, and there is no second reviewer, so requiring it blocked every merge when tried. The human read of one-way doors is a rule here and a section in every pull request, not a GitHub setting; CODEOWNERS still requests Ryan on agent-configuration changes and the CI step flags them. The agent stops at that point and asks Ryan to turn it on, then reads the protection back from the GitHub API and records what it found in the pull request. Until then, the first slice merges on the same evidence by hand.

## Rejected options

- **Agent merges on green:** removes the one human read of every one-way door.
- **Required code owner review:** blocks every merge for a single person whose agent commits under their account (tried on 2026-11-08 and removed).
- **A separate GitHub account for the agent:** would make code owner review possible, at the cost of a second account to secure for a trial project.

## Consequences

Merges wait for a person, and once protection is on, for green required checks and a code owner's review, with no administrator bypass. Renaming a CI job means updating the required checks in the same change.

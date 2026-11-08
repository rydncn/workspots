# 0007. Playwright and axe-core for end-to-end and accessibility tests

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

The task fixes this; the choice recorded is why over the Go-only alternative.

## Decision

`e2e/` is the only npm in the repository, hardened with the playbook's `.npmrc`, running `@playwright/test` with `@axe-core/playwright`.

## Rejected options

- **chromedp from Go:** no axe integration; would mean porting axe's rule engine or embedding it by hand.
- **Cypress:** no first-class JavaScript-disabled mode.

## Consequences

Two toolchains to audit; the npm one is confined to `e2e/`.

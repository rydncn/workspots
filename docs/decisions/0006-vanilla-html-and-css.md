# 0006. Vanilla HTML and CSS, no build step, no JavaScript in version one

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

Every journey must work without JavaScript, and the pages must pass WCAG 2.2 AA at 320 pixels.

## Decision

Semantic HTML from `html/template`, one hand-written stylesheet served as a static file, no script at all in version one. Progressive enhancement may be added later only for a journey that already works without it.

## Rejected options

- **htmx or Alpine:** scripts that would need CSP allowances and a second code path to test.
- **Tailwind or Sass:** a build step for a few hundred lines of CSS.

## Consequences

The CSP without `unsafe-inline` costs nothing. Revisit when a journey is measurably better with script.

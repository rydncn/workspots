# 0001. Go standard library first

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

A server-rendered site with a dozen routes. Every dependency is code the agent and reviewers must vet and keep updated.

## Decision

Go 1.27 with `net/http` and its method-and-pattern routing, `html/template` for pages, `log/slog` for logs. A dependency is added only when the standard library or a few lines cannot do the job, with an ADR saying why.

## Rejected options

- **chi, echo, gin:** `net/http` has had pattern routing with methods and wildcards since 1.22; a router adds middleware conventions without adding capability we need.
- **templ or another template compiler:** adds a build step and a generator; `html/template` already escapes by context, which is the property we rely on.

## Consequences

Less to audit; middleware is written by hand as plain `func(http.Handler) http.Handler`. Revisit if routes need per-group middleware trees that get hard to read.

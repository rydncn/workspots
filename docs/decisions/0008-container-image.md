# 0008. Distroless static image pinned by digest, non-root, no shell

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

The release rehearsal builds and scans a production image.

## Decision

Multi-stage Dockerfile: build with the official Go image, run from `gcr.io/distroless/static-debian12:nonroot` pinned by digest, a statically linked binary with `CGO_ENABLED=0`, no shell or package manager in the final image.

## Rejected options

- **alpine:** a shell and a package manager the app never uses, and musl differences to think about.
- **scratch:** no CA certificates or timezone data; distroless static carries both and a nonroot user.

## Consequences

The image can't be debugged by exec; logs and the health endpoint are the interface. Dependabot's docker ecosystem keeps the digest current.

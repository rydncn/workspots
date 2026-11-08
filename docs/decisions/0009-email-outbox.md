# 0009. Email is a development outbox; a real provider is a later decision

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

Sign-up, password change and deletion would normally send mail. The project may not sign up to any service.

## Decision

A `Sender` interface with one implementation: the development outbox, which writes each message as plain text to its own stream (not the structured log). Production configuration must name a sender; until a provider is chosen the production value is `log`, which records only the message kind and user id.

## Rejected options

- **SMTP to a local relay:** a service to run and configure that no one would read.
- **Logging the full message through slog:** would put addresses in the structured log, against AC26.

## Consequences

Choosing a provider is a new ADR and a one-way door (data leaves the server).

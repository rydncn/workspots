# 0012. argon2id from golang.org/x/crypto

**Status:** accepted · **Date:** 2026-11-08 · **Replaces:** none

## Context

Passwords must be hashed with argon2id (the api rule). The standard library has no argon2.

## Decision

`golang.org/x/crypto/argon2` IDKey with the OWASP-recommended parameters (19 MiB memory, 2 iterations, 1 lane, 16-byte salt, 32-byte key), encoded in the PHC string so parameters can change later.

## Rejected options

- **bcrypt from the same module:** the rule permits it only where argon2id isn't available, and it is.
- **A third-party argon2 package:** `x/crypto` is maintained by the Go team and is the only new module.

## Concurrency

Each argon2id computation takes 19 MiB, so a burst of sign-ins could exhaust memory. Hashing runs behind a semaphore of `runtime.NumCPU()` slots (`hashGate` in `internal/accounts`). A request that finds every slot taken waits for one; the wait ends if the client goes away or the server's write timeout cancels the request, in which case the request fails with the context's error (a 500 problem, logged) rather than being queued indefinitely. Nothing is rejected early: the rate limits in front of these routes bound the queue.

## Consequences

One module beyond pgx. The parameters are in one constant to raise later; old hashes are verified by their own PHC parameters.

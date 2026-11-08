---
paths:
  - "internal/**"
  - "cmd/**"
---

# Logging and correlation

Adapted from the playbook's `templates/rules/observability.md`. This project logs with `log/slog` through `internal/obs.NewLogger`, JSON lines to stdout.

## Structured logging

- The message is a fixed string; what varies goes in fields: `log.InfoContext(ctx, "spot approved", "spot_id", id)`, never a message built with `fmt.Sprintf`.
- Always the `…Context` methods with the request's context, so the line carries `correlation_id`. A plain `log.Info` inside a request is a bug.
- Every line has `time`, `level`, `msg`, `service` and, in a request, `correlation_id`.
- Levels: `error` needs someone to act; `warn` is unexpected but handled; `info` is a business event or the request line; `debug` (queries) is off in production.
- Never log a request body, header, query string, email address, note, token or password. Identify people by `user_id`. The query tracer logs SQL text, never arguments. `TestAC26` (slice 3) sends each of these through the app and searches the log.
- No `fmt.Println` or `log.Printf` in application code; CI greps for them.

## Correlation IDs

- `correlate` middleware takes the trace ID from an incoming W3C `traceparent`, or makes 128 random bits, puts it in the context with `obs.WithCorrelationID` and returns it as `X-Correlation-Id`. Problems carry it as `correlationId`.
- The ID travels only in `context.Context`. Any goroutine a request starts gets a context derived from the request's (`context.WithoutCancel(r.Context())` when it must outlive the response), never `context.Background()`.
- `pgx` queries carry it through the context the pool is given; `obs.QueryTracer` logs each one with it.

## How these are checked

- `TestAC25` makes one request with a `traceparent`, then asserts every log line, including the query's and (from slice 3) the mail goroutine's, carries that ID.
- CI greps application code for `fmt.Print`, `log.Print` and `context.Background()` outside `main`, tests and `internal/testdb`.

// Package obs holds the structured logger and the correlation ID that every
// log line, query and goroutine of a request carries (.claude/rules/observability.md).
package obs

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

type ctxKey struct{}

// WithCorrelationID returns ctx carrying id.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// CorrelationID returns the ID carried by ctx, or "".
func CorrelationID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// NewLogger returns a JSON logger writing to w. Every line carries the
// service name and, when the context has one, the correlation ID.
func NewLogger(w io.Writer, level slog.Leveler) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(correlating{h}).With("service", "workspots")
}

// correlating adds correlation_id from the record's context to each line.
type correlating struct{ slog.Handler }

func (c correlating) Handle(ctx context.Context, r slog.Record) error {
	if id := CorrelationID(ctx); id != "" {
		r.AddAttrs(slog.String("correlation_id", id))
	}
	return c.Handler.Handle(ctx, r)
}
func (c correlating) WithAttrs(a []slog.Attr) slog.Handler {
	return correlating{c.Handler.WithAttrs(a)}
}
func (c correlating) WithGroup(n string) slog.Handler { return correlating{c.Handler.WithGroup(n)} }

// QueryTracer logs each query at debug level with the request's context, so
// query lines carry the correlation ID. It logs the SQL text (fixed strings in
// this codebase), never the arguments, which may be personal (rule 14).
type QueryTracer struct{ Logger *slog.Logger }

type startKey struct{}

func (t QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, startKey{}, struct {
		at  time.Time
		sql string
	}{time.Now(), data.SQL})
}

func (t QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, _ := ctx.Value(startKey{}).(struct {
		at  time.Time
		sql string
	})
	attrs := []any{"sql", start.sql, "duration_ms", time.Since(start.at).Milliseconds()}
	if data.Err != nil {
		t.Logger.ErrorContext(ctx, "query", append(attrs, "error", data.Err.Error())...)
		return
	}
	t.Logger.DebugContext(ctx, "query", attrs...)
}

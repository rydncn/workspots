package web

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"github.com/rydncn/workspots/internal/obs"
)

var traceparent = regexp.MustCompile(`^00-([0-9a-f]{32})-[0-9a-f]{16}-[0-9a-f]{2}$`)

// correlate gives every request a correlation ID: the trace ID of a W3C
// traceparent header, or a new random one. It goes in the context (so every
// log line and query carries it) and in the response header.
func correlate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ""
		if m := traceparent.FindStringSubmatch(r.Header.Get("traceparent")); m != nil && m[1] != "00000000000000000000000000000000" {
			id = m[1]
		} else {
			var b [16]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Correlation-Id", id)
		next.ServeHTTP(w, r.WithContext(obs.WithCorrelationID(r.Context(), id)))
	})
}

// securityHeaders sets the headers from .claude/rules/api.md on every response.
func securityHeaders(production bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			if production {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// statusWriter records the status for the request log.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// logRequests writes one line per request: method, path, status, duration.
// Never the query string or headers, which may carry personal data (rule 14).
func (s *server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		s.log.InfoContext(r.Context(), "request", "method", r.Method, "path", r.URL.Path,
			"status", sw.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

// recover turns a panic into a 500 problem and logs it.
func (s *server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v) // the server's own way of dropping a connection
				}
				s.log.ErrorContext(r.Context(), "panic", "error", slog.AnyValue(v))
				s.writeProblem(w, r, internal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// routeErrors replaces the mux's own plain-text 404 and 405 with problems,
// keeping the Allow header on a 405. Matched routes go through the CSRF check
// and then the mux, so path values are bound.
func (s *server) routeErrors(mux *http.ServeMux) http.Handler {
	dispatch := s.csrf(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern != "" {
			dispatch.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code == http.StatusMethodNotAllowed {
			w.Header().Set("Allow", rec.Header().Get("Allow"))
			s.writeProblem(w, r, methodNotAllowed)
			return
		}
		s.writeProblem(w, r, notFound)
	})
}

package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rydncn/workspots/internal/config"
	"github.com/rydncn/workspots/internal/obs"
	"github.com/rydncn/workspots/internal/testdb"
	"github.com/rydncn/workspots/internal/web"
)

// syncBuffer collects log output from the server's goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type app struct {
	*httptest.Server
	admin *pgx.Conn // migrate role, for arranging data
	pool  *pgxpool.Pool
	logs  *syncBuffer
}

// newApp starts the real server over HTTP against its own migrated database.
func newApp(t *testing.T) *app {
	t.Helper()
	adminURL, appURL := testdb.New(t)
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close(ctx) })
	poolCfg, err := pgxpool.ParseConfig(appURL)
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	poolCfg.ConnConfig.Tracer = obs.QueryTracer{Logger: obs.NewLogger(logs, slog.LevelDebug)}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	cfg, err := config.Load(func(k string) string { return map[string]string{"DATABASE_URL": appURL}[k] })
	if err != nil {
		t.Fatal(err)
	}
	h := web.New(web.Deps{DB: pool, Config: cfg, Logger: obs.NewLogger(logs, slog.LevelDebug)})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &app{Server: srv, admin: admin, pool: pool, logs: logs}
}

func (a *app) addSpot(t *testing.T, name, status string) {
	t.Helper()
	_, err := a.admin.Exec(context.Background(),
		`INSERT INTO spots (name, kind, area, address, status) VALUES ($1, 'cafe', 'Leeds', '1 Test St', $2)`, name, status)
	if err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, a *app, path string, headers ...string) (*http.Response, string) {
	t.Helper()
	return do(t, a, http.MethodGet, path, headers...)
}

func do(t *testing.T, a *app, method, path string, headers ...string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, a.URL+path, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := a.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return res, string(body)
}

// AC1: the home page lists approved spots only.
func TestAC1HomeListsApprovedSpotsOnly(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	a.addSpot(t, "Approved Café", "approved")
	a.addSpot(t, "Second Approved", "approved")
	a.addSpot(t, "Pending Library", "pending")

	res, body := get(t, a, "/")
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	for _, want := range []string{"Approved Café", "Second Approved"} {
		if !strings.Contains(body, want) {
			t.Errorf("home page lacks %q", want)
		}
	}
	if strings.Contains(body, "Pending Library") {
		t.Error("home page shows a pending spot")
	}
	if strings.Index(body, "Approved Café") > strings.Index(body, "Second Approved") {
		t.Error("spots are not listed by name")
	}
}

type problem struct {
	Type, Title, Detail, Instance, CorrelationID string
	Status                                       int
}

func readProblem(t *testing.T, res *http.Response, body string) problem {
	t.Helper()
	if ct := res.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type %q, body %q", ct, body)
	}
	var p problem
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatalf("not JSON: %v: %q", err, body)
	}
	if p.Detail == "" || p.Title == "" || p.CorrelationID == "" {
		t.Errorf("problem lacks a required field: %+v", p)
	}
	return p
}

// AC23: every error is a problem, JSON or HTML by Accept, revealing nothing on a 500.
func TestAC23EveryErrorIsAProblem(t *testing.T) {
	t.Parallel()
	a := newApp(t)

	t.Run("unknown path as JSON", func(t *testing.T) {
		res, body := get(t, a, "/no/such/page", "Accept", "application/json")
		p := readProblem(t, res, body)
		if res.StatusCode != 404 || p.Status != 404 || p.Type != "/problems/not-found" || p.Title == "" || p.Instance != "/no/such/page" {
			t.Errorf("got %d %+v", res.StatusCode, p)
		}
		if p.CorrelationID == "" || p.CorrelationID != res.Header.Get("X-Correlation-Id") {
			t.Errorf("correlation id %q vs header %q", p.CorrelationID, res.Header.Get("X-Correlation-Id"))
		}
	})
	t.Run("unknown path as HTML", func(t *testing.T) {
		res, body := get(t, a, "/no/such/page", "Accept", "text/html,application/xhtml+xml")
		if res.StatusCode != 404 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
			t.Fatalf("got %d %s", res.StatusCode, res.Header.Get("Content-Type"))
		}
		if !strings.Contains(body, "<title>") || !strings.Contains(body, res.Header.Get("X-Correlation-Id")) || !strings.Contains(body, "404") {
			t.Errorf("HTML problem page lacks title, status or correlation id: %q", body)
		}
	})
	t.Run("missing static file and directory listing", func(t *testing.T) {
		for _, path := range []string{"/static/missing.css", "/static/", "/static"} {
			res, body := get(t, a, path, "Accept", "application/json")
			p := readProblem(t, res, body)
			if res.StatusCode != 404 || p.Type != "/problems/not-found" {
				t.Errorf("%s: got %d %+v", path, res.StatusCode, p)
			}
		}
		if res, _ := get(t, a, "/static/style.css"); res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/css") {
			t.Errorf("stylesheet: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
		}
	})
	t.Run("wrong method", func(t *testing.T) {
		res, body := do(t, a, http.MethodPost, "/", "Accept", "application/json")
		p := readProblem(t, res, body)
		if res.StatusCode != 405 || p.Type != "/problems/method-not-allowed" || !strings.Contains(res.Header.Get("Allow"), "GET") {
			t.Errorf("got %d %+v allow=%q", res.StatusCode, p, res.Header.Get("Allow"))
		}
	})
	t.Run("unexpected failure", func(t *testing.T) {
		b := newApp(t)
		b.pool.Close() // every query now fails
		res, body := get(t, b, "/", "Accept", "application/json")
		p := readProblem(t, res, body)
		if res.StatusCode != 500 || p.Type != "/problems/internal" || p.CorrelationID == "" {
			t.Errorf("got %d %+v", res.StatusCode, p)
		}
		for _, leak := range []string{"closed", "pool", "pgx", "SQL", "select"} {
			if strings.Contains(strings.ToLower(body), strings.ToLower(leak)) {
				t.Errorf("500 body leaks %q: %s", leak, body)
			}
		}
	})
}

// AC24: security headers on every response, including errors.
func TestAC24SecurityHeaders(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	for _, path := range []string{"/", "/no/such/page"} {
		res, _ := get(t, a, path)
		h := res.Header
		csp := h.Get("Content-Security-Policy")
		for _, want := range []string{"default-src 'self'", "object-src 'none'", "base-uri 'none'", "frame-ancestors 'none'", "form-action 'self'"} {
			if !strings.Contains(csp, want) {
				t.Errorf("%s: CSP %q lacks %q", path, csp, want)
			}
		}
		if strings.Contains(csp, "unsafe-") {
			t.Errorf("%s: CSP allows unsafe: %q", path, csp)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
			t.Errorf("%s: headers %v", path, h)
		}
		if h.Get("Strict-Transport-Security") != "" {
			t.Errorf("%s: HSTS set in development", path)
		}
	}
	// Production: the same handler with a production config (no database needed for a 404).
	prod := httptest.NewServer(web.New(web.Deps{Config: config.Config{Env: "production"}, Logger: obs.NewLogger(io.Discard, slog.LevelInfo)}))
	defer prod.Close()
	res, err := prod.Client().Get(prod.URL + "/no/such/page")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if !strings.HasPrefix(res.Header.Get("Strict-Transport-Security"), "max-age=") {
		t.Errorf("production response lacks HSTS: %v", res.Header)
	}
}

var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// AC25: one correlation ID per request, in the header and on every log line
// the request produced, including the database query's; traceparent honoured.
func TestAC25CorrelationIDEverywhere(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	a.addSpot(t, "Logged Café", "approved")

	res, _ := get(t, a, "/", "traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	if got := res.Header.Get("X-Correlation-Id"); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("correlation header %q", got)
	}
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(a.logs.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", raw)
		}
		lines = append(lines, m)
	}
	var sawQuery, sawRequest bool
	for _, m := range lines {
		if m["correlation_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Errorf("log line without the request's correlation id: %v", m)
		}
		switch m["msg"] {
		case "query":
			sawQuery = true
		case "request":
			sawRequest = true
		}
	}
	if !sawQuery || !sawRequest || len(lines) < 2 {
		t.Errorf("expected a query line and a request line, got %v", lines)
	}

	res, _ = get(t, a, "/")
	if got := res.Header.Get("X-Correlation-Id"); !hex32.MatchString(got) {
		t.Errorf("generated correlation id %q", got)
	}
}

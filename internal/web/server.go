// Package web is the only package that knows about HTTP: routes, handlers,
// templates and middleware. Domain packages return errors; this package turns
// them into pages and problem details (.claude/rules/api.md).
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rydncn/workspots/internal/config"
	"github.com/rydncn/workspots/internal/mail"
	"github.com/rydncn/workspots/internal/spots"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var staticFiles = mustSub(staticFS, "static")

// Deps is what the handlers need.
type Deps struct {
	DB     *pgxpool.Pool
	Config config.Config
	Logger *slog.Logger
	Mail   mail.Sender
}

type server struct {
	db     *pgxpool.Pool
	cfg    config.Config
	log    *slog.Logger
	mail   mail.Sender
	limits *limiter
	tpl    map[string]*template.Template
}

// New returns the application's handler.
func New(d Deps) http.Handler {
	s := &server{db: d.DB, cfg: d.Config, log: d.Logger, mail: d.Mail, limits: newLimiter(15 * time.Minute), tpl: parseTemplates()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /search", s.search)
	mux.HandleFunc("GET /spots/{id}", s.spot)
	mux.HandleFunc("GET /static/{file...}", s.static)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /sign-up", s.signUpForm)
	mux.HandleFunc("POST /sign-up", s.signUpPost)
	mux.HandleFunc("GET /sign-in", s.signInForm)
	mux.HandleFunc("POST /sign-in", s.signInPost)
	mux.HandleFunc("POST /sign-out", s.signOut)
	mux.HandleFunc("GET /account", s.account)
	mux.HandleFunc("GET /account/password", s.passwordForm)
	mux.HandleFunc("POST /account/password", s.passwordPost)

	var h http.Handler = s.routeErrors(mux)
	h = s.sessions(h)
	h = s.recover(h)
	h = s.logRequests(h)
	h = securityHeaders(d.Config.Production())(h)
	return correlate(h)
}

// static serves one embedded file; a missing file or a directory is a problem,
// not the file server's plain-text 404 or a listing (AC23).
func (s *server) static(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	info, err := fs.Stat(staticFiles, name)
	if err != nil || info.IsDir() {
		s.writeProblem(w, r, notFound)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFileFS(w, r, staticFiles, name)
}

// health answers 200 "ok" when the database answers, else a 503 problem; for
// the container's health check and the release rehearsal (ADR 0008).
func (s *server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := s.db.Ping(r.Context()); err != nil {
		s.log.ErrorContext(r.Context(), "health check failed", "error", err.Error())
		s.writeProblem(w, r, unavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	list, err := spots.ListApproved(r.Context(), s.db)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "home.html", map[string]any{"Spots": list, "Form": searchForm{}, "WifiLabels": spots.WifiLabels, "NoiseLabels": spots.NoiseLabels})
}

// fail logs err with the request's correlation ID and answers with its problem.
func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	p := problemFor(err)
	if p.Status >= 500 {
		s.log.ErrorContext(r.Context(), "request failed", "error", err.Error())
	}
	s.writeProblem(w, r, p)
}

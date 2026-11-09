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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rydncn/workspots/internal/config"
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
}

type server struct {
	db  *pgxpool.Pool
	cfg config.Config
	log *slog.Logger
	tpl map[string]*template.Template
}

// New returns the application's handler.
func New(d Deps) http.Handler {
	s := &server{db: d.DB, cfg: d.Config, log: d.Logger, tpl: parseTemplates()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /static/{file...}", s.static)

	var h http.Handler = s.routeErrors(mux)
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

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	list, err := spots.ListApproved(r.Context(), s.db)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "home.html", map[string]any{"Spots": list})
}

// fail logs err with the request's correlation ID and answers with its problem.
func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	p := problemFor(err)
	if p.Status >= 500 {
		s.log.ErrorContext(r.Context(), "request failed", "error", err.Error())
	}
	s.writeProblem(w, r, p)
}

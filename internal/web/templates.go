package web

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"path"
)

// parseTemplates builds one template set per page, each sharing layout.html.
func parseTemplates() map[string]*template.Template {
	pages, _ := fs.Glob(templateFS, "templates/*.html")
	out := map[string]*template.Template{}
	for _, p := range pages {
		name := path.Base(p)
		if name == "layout.html" {
			continue
		}
		out[name] = template.Must(template.ParseFS(templateFS, "templates/layout.html", p))
	}
	return out
}

// render executes the page into a buffer first, so a template error becomes a
// problem response rather than half a page.
func (s *server) render(w http.ResponseWriter, r *http.Request, status int, page string, data any) {
	var buf bytes.Buffer
	if err := s.tpl[page].ExecuteTemplate(&buf, "layout.html", data); err != nil {
		s.log.ErrorContext(r.Context(), "template", "page", page, "error", err.Error())
		if page != "problem.html" {
			s.writeProblem(w, r, internal)
		}
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

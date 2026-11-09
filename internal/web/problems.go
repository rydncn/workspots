package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rydncn/workspots/internal/obs"
	"github.com/rydncn/workspots/internal/spots"
)

// Problem is an RFC 9457 problem. Type is a stable path per kind of problem.
type Problem struct {
	Type          string       `json:"type"`
	Title         string       `json:"title"`
	Status        int          `json:"status"`
	Detail        string       `json:"detail,omitempty"`
	Instance      string       `json:"instance,omitempty"`
	CorrelationID string       `json:"correlationId,omitempty"`
	Errors        []fieldError `json:"errors,omitempty"`
}

var (
	notFound         = Problem{Type: "/problems/not-found", Title: "Not found", Status: 404, Detail: "There is nothing at this address."}
	methodNotAllowed = Problem{Type: "/problems/method-not-allowed", Title: "Method not allowed", Status: 405, Detail: "This address does not accept that method; see the Allow header."}
	internal         = Problem{Type: "/problems/internal", Title: "Something went wrong", Status: 500,
		Detail: "The problem has been logged. Quote the correlation ID if you report it."}
	crossSite = Problem{Type: "/problems/csrf", Title: "Request refused", Status: 403,
		Detail: "This form was sent without a valid csrf token, or from another site. Go back, reload the page and try again."}
	badForm = Problem{Type: "/problems/bad-form", Title: "Form could not be read", Status: 400,
		Detail: "The form data was missing, too large or malformed."}
	signInRequired = Problem{Type: "/problems/sign-in-required", Title: "Sign in required", Status: 401,
		Detail: "Sign in to do this."}
	validation = Problem{Type: "/problems/validation", Title: "Some fields need attention", Status: 422,
		Detail: "One or more fields are missing or not valid; see errors."}
	tooMany = Problem{Type: "/problems/rate-limited", Title: "Too many attempts", Status: 429,
		Detail: "Too many attempts in a short time. Wait for the number of seconds in the Retry-After header and try again."}
)

// domainProblems maps each domain error to its problem, in one place. Domain
// packages know nothing about HTTP; the edge decides the status.
var domainProblems = []struct {
	err     error
	problem Problem
}{
	{spots.ErrSpotNotFound, Problem{Type: "/problems/spot-not-found", Title: "Spot not found", Status: 404, Detail: "There is no spot at this address, or it is awaiting approval."}},
}

// problemFor turns any error into a problem: a mapped domain error into its
// own, anything else into the generic 500 (its detail goes to the log only).
func problemFor(err error) Problem {
	for _, m := range domainProblems {
		if errors.Is(err, m.err) {
			return m.problem
		}
	}
	return internal
}

// writeProblem renders p as JSON, or as an HTML page when the request prefers
// HTML (a browser), with the same fields in both.
func (s *server) writeProblem(w http.ResponseWriter, r *http.Request, p Problem) {
	p.Instance = r.URL.Path
	p.CorrelationID = obs.CorrelationID(r.Context())
	if prefersHTML(r) {
		s.render(w, r, p.Status, "problem.html", map[string]any{"Problem": p})
		return
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// prefersHTML is true when text/html appears in Accept before application/json
// (browsers send text/html first; API clients send JSON or nothing).
func prefersHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	html, js := strings.Index(accept, "text/html"), strings.Index(accept, "application/json")
	return html >= 0 && (js < 0 || html < js)
}

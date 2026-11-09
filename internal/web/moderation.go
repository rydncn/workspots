package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/rydncn/workspots/internal/accounts"
	"github.com/rydncn/workspots/internal/moderation"
	"github.com/rydncn/workspots/internal/spots"
)

// requireRole answers a request from anyone below the role: a 401 problem
// signed out, a 403 problem otherwise (AC18, AC21). Moderation pages are not
// linked for members, so there is no sign-in redirect to come back from.
func (s *server) requireRole(w http.ResponseWriter, r *http.Request, roles ...string) (accounts.User, bool) {
	u := currentSession(r).user
	if u == nil {
		s.writeProblem(w, r, signInRequired)
		return accounts.User{}, false
	}
	if !contains(roles, u.Role) {
		s.fail(w, r, accounts.ErrPermissionDenied)
		return accounts.User{}, false
	}
	return *u, true
}

func (s *server) requireModerator(w http.ResponseWriter, r *http.Request) (accounts.User, bool) {
	return s.requireRole(w, r, "moderator", "admin")
}

// queue is GET /moderation: pending spots and open problems (AC19, AC20, AC22).
func (s *server) queue(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireModerator(w, r); !ok {
		return
	}
	s.renderQueue(w, r, http.StatusOK, nil)
}

func (s *server) renderQueue(w http.ResponseWriter, r *http.Request, status int, errs []fieldError) {
	pending, err := spots.Pending(r.Context(), s.db)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	problems, err := moderation.OpenProblems(r.Context(), s.db)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	data := map[string]any{"Pending": pending, "Problems": problems}
	if errs != nil {
		s.invalid(w, r, "moderation.html", data, errs)
		return
	}
	s.render(w, r, status, "moderation.html", data)
}

// moderate runs one moderation action on the id in the path, then returns to
// the queue. Moderators' actions are not rate limited.
func (s *server) moderate(act func(r *http.Request, actor, id int64) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.requireModerator(w, r)
		if !ok {
			return
		}
		id, ok := pathID(r, "id")
		if !ok {
			s.fail(w, r, moderation.ErrTargetNotFound)
			return
		}
		if err := act(r, u.ID, id); err != nil {
			s.fail(w, r, err)
			return
		}
		http.Redirect(w, r, "/moderation", http.StatusSeeOther)
	}
}

func (s *server) setStatus(status string) http.HandlerFunc {
	return s.moderate(func(r *http.Request, actor, id int64) error {
		return moderation.SetStatus(r.Context(), s.db, actor, id, status)
	})
}

func (s *server) hideReport(w http.ResponseWriter, r *http.Request) {
	reason := field(r, "reason")
	if reason == "" || tooLong(reason, maxReason) {
		if _, ok := s.requireModerator(w, r); !ok {
			return
		}
		s.renderQueue(w, r, http.StatusUnprocessableEntity, []fieldError{{"reason", "Say why the report is hidden, in at most 500 characters."}})
		return
	}
	s.moderate(func(r *http.Request, actor, id int64) error {
		return moderation.HideReport(r.Context(), s.db, actor, id, reason)
	})(w, r)
}

func (s *server) resolveProblem(w http.ResponseWriter, r *http.Request) {
	s.moderate(func(r *http.Request, actor, id int64) error { return moderation.Resolve(r.Context(), s.db, actor, id) })(w, r)
}

// usersForm is GET /admin/users: set the role of an account by its address (AC21).
func (s *server) usersForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireRole(w, r, "admin"); !ok {
		return
	}
	s.render(w, r, http.StatusOK, "admin-users.html", map[string]any{"Email": "", "Roles": accounts.Roles})
}

func (s *server) usersPost(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireRole(w, r, "admin")
	if !ok {
		return
	}
	email, role := field(r, "email"), field(r, "role")
	data := map[string]any{"Email": email, "Roles": accounts.Roles}
	var errs []fieldError
	if accounts.ValidateEmail(email) != nil {
		errs = append(errs, fieldError{"email", "Enter the account's email address."})
	}
	if !contains(accounts.Roles, role) {
		errs = append(errs, fieldError{"role", "Choose a role: " + strings.Join(accounts.Roles, ", ") + "."})
	}
	if len(errs) == 0 {
		switch err := accounts.SetRole(r.Context(), s.db, u.ID, email, role); {
		case errors.Is(err, accounts.ErrUserNotFound):
			errs = append(errs, fieldError{"email", "No account has that address."})
		case err != nil:
			s.fail(w, r, err)
			return
		}
	}
	if len(errs) > 0 {
		s.invalid(w, r, "admin-users.html", data, errs)
		return
	}
	data["Changed"] = role
	s.render(w, r, http.StatusOK, "admin-users.html", data)
}

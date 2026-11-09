package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rydncn/workspots/internal/accounts"
	"github.com/rydncn/workspots/internal/mail"
)

// Rate limits (AC12): failures per address and per IP for sign-in, attempts
// per IP for sign-up, failures per account for password change, in 15 minutes.
const (
	signInPerAddress = 5
	signInPerIP      = 10
	signUpPerIP      = 10
	changePerUser    = 5
)

type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// requireUser answers a signed-out request: a redirect to sign in for GET,
// a 401 problem otherwise. Returns false when the caller should stop.
func (s *server) requireUser(w http.ResponseWriter, r *http.Request) (accounts.User, bool) {
	sess := currentSession(r)
	if sess.user != nil {
		return *sess.user, true
	}
	if r.Method == http.MethodGet {
		http.Redirect(w, r, "/sign-in?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
	} else {
		s.writeProblem(w, r, signInRequired)
	}
	return accounts.User{}, false
}

func (s *server) signUpForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "sign-up.html", map[string]any{"Email": ""})
}

func (s *server) signUpPost(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.PostForm.Get("email"))
	password := r.PostForm.Get("password")
	var errs []fieldError
	if err := accounts.ValidateEmail(email); err != nil {
		errs = append(errs, fieldError{"email", err.Error()})
	}
	if err := accounts.ValidatePassword(password); err != nil {
		errs = append(errs, fieldError{"password", err.Error()})
	}
	if len(errs) > 0 {
		s.invalid(w, r, "sign-up.html", map[string]any{"Email": email}, errs)
		return
	}
	if wait := s.limits.take("signup:ip:"+s.clientIP(r), signUpPerIP); wait > 0 {
		s.rateLimited(w, r, wait)
		return
	}
	created, err := accounts.SignUp(r.Context(), s.db, email, password)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if created {
		s.send(r, mail.Message{Kind: "welcome", To: email, Subject: "Welcome to Workspots",
			Body: "Your Workspots account is ready. Sign in at " + s.cfg.BaseURL + "/sign-in"})
	} else {
		s.send(r, mail.Message{Kind: "already-registered", To: email, Subject: "You already have a Workspots account",
			Body: "Someone tried to sign up with this address, but you already have an account. Sign in at " + s.cfg.BaseURL + "/sign-in"})
	}
	s.render(w, r, http.StatusOK, "check-email.html", map[string]any{})
}

func (s *server) signInForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "sign-in.html", map[string]any{"Email": "", "Next": safeNext(r.URL.Query().Get("next"))})
}

func (s *server) signInPost(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.PostForm.Get("email"))
	password := r.PostForm.Get("password")
	next := safeNext(r.PostForm.Get("next"))
	data := map[string]any{"Email": email, "Next": next}
	if email == "" || password == "" {
		s.invalid(w, r, "sign-in.html", data, []fieldError{{"", "Enter your email address and password."}})
		return
	}
	addrKey, ipKey := "signin:addr:"+strings.ToLower(email), "signin:ip:"+s.clientIP(r)
	if wait := max(s.limits.take(addrKey, signInPerAddress), s.limits.take(ipKey, signInPerIP)); wait > 0 {
		s.rateLimited(w, r, wait)
		return
	}
	u, err := accounts.Authenticate(r.Context(), s.db, email, password)
	if errors.Is(err, accounts.ErrInvalidCredentials) {
		s.invalid(w, r, "sign-in.html", data, []fieldError{{"", accounts.ErrInvalidCredentials.Error()}})
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.limits.refund(addrKey)
	s.limits.refund(ipKey)
	if err := s.signIn(w, r, u); err != nil {
		s.fail(w, r, err)
		return
	}
	if next == "/" {
		next = "/account"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *server) signOut(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	if err := accounts.EndSession(r.Context(), s.db, currentSession(r).token); err != nil {
		s.fail(w, r, err)
		return
	}
	setCookie(w, sessionCookie, "", -1)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) account(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	s.render(w, r, http.StatusOK, "account.html", map[string]any{"Account": u, "Changed": r.URL.Query().Get("changed") == "1"})
}

func (s *server) passwordForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	s.render(w, r, http.StatusOK, "password.html", map[string]any{})
}

func (s *server) passwordPost(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	key, ipKey := "pwchange:user:"+itoa(u.ID), "pwchange:ip:"+s.clientIP(r)
	if wait := max(s.limits.take(key, changePerUser), s.limits.take(ipKey, signInPerIP)); wait > 0 {
		s.rateLimited(w, r, wait)
		return
	}
	err := accounts.ChangePassword(r.Context(), s.db, u.ID, r.PostForm.Get("current"), r.PostForm.Get("password"))
	switch {
	case errors.Is(err, accounts.ErrInvalidCredentials):
		s.invalid(w, r, "password.html", map[string]any{}, []fieldError{{"current", "The current password is not right."}})
		return
	case errors.Is(err, accounts.ErrPasswordTooShort), errors.Is(err, accounts.ErrPasswordTooLong):
		s.invalid(w, r, "password.html", map[string]any{}, []fieldError{{"password", err.Error()}})
		return
	case err != nil:
		s.fail(w, r, err)
		return
	}
	s.limits.refund(key)
	s.limits.refund(ipKey)
	if err := accounts.EndOtherSessions(r.Context(), s.db, u.ID, currentSession(r).token); err != nil {
		s.fail(w, r, err)
		return
	}
	s.send(r, mail.Message{Kind: "password-changed", To: u.Email, Subject: "Your Workspots password was changed",
		Body: "Your password was just changed and your other sessions were signed out. If this was not you, change it again at " + s.cfg.BaseURL + "/account/password"})
	http.Redirect(w, r, "/account?changed=1", http.StatusSeeOther)
}

// send delivers a message in a goroutine that outlives the response but keeps
// the request's context values, so its log line carries the correlation ID.
func (s *server) send(r *http.Request, m mail.Message) {
	ctx := context.WithoutCancel(r.Context())
	go func() {
		defer func() { // a panicking provider must not take the process down
			if v := recover(); v != nil {
				s.log.ErrorContext(ctx, "mail panic", "kind", m.Kind, "error", fmt.Sprint(v))
			}
		}()
		if err := s.mail.Send(ctx, m); err != nil {
			s.log.ErrorContext(ctx, "mail failed", "kind", m.Kind, "error", err.Error())
			return
		}
		s.log.InfoContext(ctx, "mail sent", "kind", m.Kind)
	}()
}

// invalid re-renders a form with its errors at 422, or answers a validation
// problem for clients that don't want HTML.
func (s *server) invalid(w http.ResponseWriter, r *http.Request, page string, data map[string]any, errs []fieldError) {
	if !prefersHTML(r) {
		p := validation
		p.Errors = errs
		s.writeProblem(w, r, p)
		return
	}
	data["Errors"] = errs
	s.render(w, r, http.StatusUnprocessableEntity, page, data)
}

func (s *server) rateLimited(w http.ResponseWriter, r *http.Request, wait int) {
	w.Header().Set("Retry-After", retryHeader(wait))
	s.writeProblem(w, r, tooMany)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

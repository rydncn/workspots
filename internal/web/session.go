package web

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rydncn/workspots/internal/accounts"
)

const (
	sessionCookie = "__Host-session"
	csrfCookie    = "__Host-csrf"
)

type sessionKey struct{}

// session is what the middleware learned about the request's cookies.
type session struct {
	user       *accounts.User // nil when signed out
	token      string         // the session cookie's value, "" when signed out
	csrfSecret string         // the __Host-csrf cookie's value, always set
}

func (s *session) id() string {
	if s.token == "" {
		return "anonymous"
	}
	return accounts.SessionID(s.token)
}

// csrfToken is the form token: an HMAC of the session ID keyed by the secret
// in the browser's own cookie, so it is tied to this session once one exists
// and to this browser before (.claude/rules/api.md).
func (s *session) csrfToken() string {
	mac := hmac.New(sha256.New, []byte(s.csrfSecret))
	mac.Write([]byte(s.id()))
	return hex.EncodeToString(mac.Sum(nil))
}

func currentSession(r *http.Request) *session {
	s, _ := r.Context().Value(sessionKey{}).(*session)
	if s == nil {
		return &session{}
	}
	return s
}

func newSecret() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// setCookie writes a __Host- cookie: Secure, HttpOnly, SameSite=Lax, Path=/,
// no Domain. maxAge < 0 clears it.
func setCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: maxAge,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// sessions resolves the cookies into a session on the context, issuing a
// CSRF secret when the browser has none.
func (s *server) sessions(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r) // publicly cacheable, so never a Set-Cookie
			return
		}
		sess := &session{}
		if c, err := r.Cookie(csrfCookie); err == nil && len(c.Value) == 64 {
			sess.csrfSecret = c.Value
		} else {
			sess.csrfSecret = newSecret()
			setCookie(w, csrfCookie, sess.csrfSecret, int(accounts.SessionLifetime.Seconds()))
		}
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			u, err := accounts.SessionUser(r.Context(), s.db, c.Value)
			switch {
			case err == nil:
				sess.user, sess.token = &u, c.Value
			case err == accounts.ErrSessionNotFound:
				setCookie(w, sessionCookie, "", -1)
			default:
				s.fail(w, r, err)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, sess)))
	})
}

// csrf refuses any POST that is cross-site or lacks this session's token.
func (s *server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !sameOrigin(r) {
				s.writeProblem(w, r, crossSite)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
			if err := r.ParseForm(); err != nil {
				s.writeProblem(w, r, badForm)
				return
			}
			want := currentSession(r).csrfToken()
			if !subtleEqual(r.PostForm.Get("csrf"), want) {
				s.writeProblem(w, r, crossSite)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func subtleEqual(a, b string) bool { return hmac.Equal([]byte(a), []byte(b)) }

// sameOrigin: an Origin header must name this host, and Sec-Fetch-Site must
// be same-origin (or none, a direct navigation). A POST with neither header is
// refused: every current browser sends at least one, and the forms are only
// ever posted by browsers.
func sameOrigin(r *http.Request) bool {
	origin, site := r.Header.Get("Origin"), r.Header.Get("Sec-Fetch-Site")
	if origin == "" && site == "" {
		return false
	}
	if origin != "" {
		u, err := url.Parse(origin)
		if err != nil || origin == "null" || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	return site == "" || site == "same-origin" || site == "none"
}

// signIn starts a session for u, ending the one the browser had (AC8), and
// rotates the CSRF secret.
func (s *server) signIn(w http.ResponseWriter, r *http.Request, u accounts.User) error {
	sess := currentSession(r)
	if sess.token != "" {
		if err := accounts.EndSession(r.Context(), s.db, sess.token); err != nil {
			return err
		}
	}
	token, err := accounts.CreateSession(r.Context(), s.db, u.ID)
	if err != nil {
		return err
	}
	setCookie(w, sessionCookie, token, int(accounts.SessionLifetime.Seconds()))
	setCookie(w, csrfCookie, newSecret(), int(accounts.SessionLifetime.Seconds()))
	return nil
}

// clientIP is the peer address, or the last X-Forwarded-For hop when the peer
// is the trusted proxy.
func (s *server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if s.cfg.TrustedProxy != "" && host == s.cfg.TrustedProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	return host
}

// safeNext returns next if it is one of our own paths, else "/".
func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.ContainsAny(next, "\\\r\n") {
		return next
	}
	return "/"
}

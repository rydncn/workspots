package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// AC7: sign-up creates a member and mails a welcome; a taken address gets the
// same page and a different mail; bad passwords are 422 with the email kept.
func TestAC7SignUp(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	b := a.browser(t)
	email := unique("ac7")

	res, body := b.post("/sign-up", "/sign-up", url.Values{"email": {email}, "password": {goodPassword}})
	if res.StatusCode != 200 || !strings.Contains(body, "Check your email, then sign in") {
		t.Fatalf("sign-up: %d %q", res.StatusCode, body)
	}
	var role string
	if err := a.admin.QueryRow(context.Background(), `SELECT role FROM users WHERE lower(email) = lower($1)`, email).Scan(&role); err != nil || role != "member" {
		t.Fatalf("user row: %q %v", role, err)
	}
	if b.cookie("__Host-session") != nil {
		t.Error("sign-up signed the visitor in")
	}

	// The same address again, differently cased: no second account, same page, a different mail.
	res, body2 := b.post("/sign-up", "/sign-up", url.Values{"email": {strings.ToUpper(email)}, "password": {goodPassword}})
	if res.StatusCode != 200 || body2 != body {
		t.Errorf("taken address: %d, page differs from the first", res.StatusCode)
	}
	var n int
	_ = a.admin.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE lower(email) = lower($1)`, email).Scan(&n)
	if n != 1 {
		t.Errorf("%d accounts for the address", n)
	}
	kinds := waitForMail(t, a, 2)
	if kinds[0] != "welcome" || kinds[1] != "already-registered" {
		t.Errorf("mail kinds %v", kinds)
	}
	for _, m := range a.mail.Sent() {
		if !strings.EqualFold(m.To, email) {
			t.Errorf("mail to %q", m.To)
		}
	}

	for _, pw := range []string{"short", strings.Repeat("x", 1025)} {
		res, body := b.post("/sign-up", "/sign-up", url.Values{"email": {"keep@example.test"}, "password": {pw}})
		if res.StatusCode != 422 || !strings.Contains(body, `value="keep@example.test"`) || !strings.Contains(body, "12") {
			t.Errorf("password %d chars: %d, email kept=%v", len(pw), res.StatusCode, strings.Contains(body, `value="keep@example.test"`))
		}
	}
	// Validation as JSON is a problem with an errors list of field and message.
	_, page := b.get("/sign-up")
	token := csrfField.FindStringSubmatch(page)[1]
	res2, body2 := b.postRaw("/sign-up", url.Values{"email": {"x"}, "password": {"short"}, "csrf": {token}}, "Origin", a.URL, "Accept", "application/json")
	var p struct {
		Status int
		Errors []map[string]string
	}
	if err := json.Unmarshal([]byte(body2), &p); err != nil || res2.StatusCode != 422 || len(p.Errors) != 2 || p.Errors[0]["field"] != "email" || p.Errors[1]["message"] == "" {
		t.Errorf("JSON validation: %d %s", res2.StatusCode, body2)
	}
}

func waitForMail(t *testing.T, a *app, n int) []string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if sent := a.mail.Sent(); len(sent) >= n {
			kinds := make([]string, len(sent))
			for i, m := range sent {
				kinds[i] = m.Kind
			}
			return kinds
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected %d mails, got %d", n, len(a.mail.Sent()))
	return nil
}

// AC8: a correct sign-in sets a __Host- cookie that differs from any earlier
// one and ends the earlier session; wrong password and unknown address are the
// same 422 with no cookie.
func TestAC8SignIn(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	b := a.browser(t)
	email := unique("ac8")
	b.signUp(email, goodPassword)

	res, _ := b.signIn(email, goodPassword)
	if res.StatusCode != 303 || res.Header.Get("Location") != "/account" {
		t.Fatalf("sign-in: %d -> %q", res.StatusCode, res.Header.Get("Location"))
	}
	c := b.cookie("__Host-session")
	if c == nil {
		t.Fatal("no session cookie")
	}
	raw := res.Header.Values("Set-Cookie")
	var sessionSet string
	for _, h := range raw {
		if strings.HasPrefix(h, "__Host-session=") {
			sessionSet = h
		}
	}
	for _, want := range []string{"Secure", "HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(sessionSet, want) {
			t.Errorf("session cookie lacks %s: %s", want, sessionSet)
		}
	}
	if strings.Contains(sessionSet, "Domain=") {
		t.Errorf("session cookie has a Domain: %s", sessionSet)
	}
	var csrfSet string
	for _, h := range raw {
		if strings.HasPrefix(h, "__Host-csrf=") {
			csrfSet = h
		}
	}
	for _, want := range []string{"Secure", "HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(csrfSet, want) {
			t.Errorf("csrf cookie lacks %s (sign-in must rotate it): %s", want, csrfSet)
		}
	}
	_, body := b.get("/account")
	if !strings.Contains(body, email) {
		t.Error("account page does not show the address")
	}
	// Retention: an expired session row is deleted at the next sign-in, not just ignored.
	if _, err := a.admin.Exec(context.Background(), `INSERT INTO sessions (id, user_id, expires_at)
		SELECT 'expired-row', id, now() - interval '1 day' FROM users WHERE lower(email) = lower($1)`, email); err != nil {
		t.Fatal(err)
	}

	// Signing in again rotates the session and ends the old one.
	first := c.Value
	res, _ = b.signIn(email, goodPassword)
	second := b.cookie("__Host-session").Value
	if res.StatusCode != 303 || second == first {
		t.Errorf("session not rotated: %d, same=%v", res.StatusCode, second == first)
	}
	var live int
	_ = a.admin.QueryRow(context.Background(), `SELECT count(*) FROM sessions s JOIN users u ON u.id = s.user_id WHERE lower(u.email) = lower($1) AND s.expires_at > now()`, email).Scan(&live)
	if live != 1 {
		t.Errorf("%d live sessions after re-signing in, want 1", live)
	}
	var expired int
	_ = a.admin.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE id = 'expired-row'`).Scan(&expired)
	if expired != 0 {
		t.Error("expired session row was not deleted")
	}

	// Refusals look alike.
	wrong := a.browser(t)
	res1, body1 := wrong.signIn(email, "not the password 123")
	res2, body2 := wrong.signIn(unique("nobody"), goodPassword)
	if res1.StatusCode != 422 || res2.StatusCode != 422 {
		t.Fatalf("refusals: %d %d", res1.StatusCode, res2.StatusCode)
	}
	msg := func(b string) string {
		i := strings.Index(b, `class="error"`)
		if i < 0 {
			return ""
		}
		return b[i : i+120]
	}
	if msg(body1) == "" || msg(body1) != msg(body2) {
		t.Errorf("refusal messages differ or are missing:\n%s\n%s", msg(body1), msg(body2))
	}
	if wrong.cookie("__Host-session") != nil {
		t.Error("a refused sign-in set a session cookie")
	}

	// A return path that a browser would turn into another site is replaced.
	evil := a.browser(t)
	res, _ = evil.post("/sign-in?next=/%09/evil.example", "/sign-in", url.Values{"email": {email}, "password": {goodPassword}})
	if loc := res.Header.Get("Location"); res.StatusCode != 303 || loc != "/account" {
		t.Errorf("tab in next: %d -> %q", res.StatusCode, loc)
	}
}

// AC9: sign-out ends the session on the server; replaying the cookie is signed out.
func TestAC9SignOut(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	b := a.browser(t)
	email := unique("ac9")
	b.signUp(email, goodPassword)
	b.signIn(email, goodPassword)
	old := b.cookie("__Host-session")

	res, _ := b.post("/account", "/sign-out", url.Values{})
	if res.StatusCode != 303 || b.cookie("__Host-session") != nil {
		t.Fatalf("sign-out: %d, cookie still present=%v", res.StatusCode, b.cookie("__Host-session") != nil)
	}
	replay := a.browser(t) // its own client, which does not follow redirects
	req, _ := http.NewRequest(http.MethodGet, a.URL+"/account", nil)
	req.AddCookie(old)
	res2, _ := replay.do(req)
	if res2.StatusCode != 303 || !strings.HasPrefix(res2.Header.Get("Location"), "/sign-in") {
		t.Errorf("replayed cookie: %d -> %q", res2.StatusCode, res2.Header.Get("Location"))
	}
}

// AC10: change password needs the current one, ends other sessions, mails.
func TestAC10ChangePassword(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	email := unique("ac10")
	b := a.browser(t)
	b.signUp(email, goodPassword)
	b.signIn(email, goodPassword)
	other := a.browser(t)
	other.signIn(email, goodPassword)

	const newPassword = "a different long password"
	// AC7: the length is checked before any hashing, so an over-long new
	// password is refused as such even when the current one is wrong.
	res, body := b.post("/account/password", "/account/password", url.Values{"current": {"wrong current pw"}, "password": {strings.Repeat("x", 1025)}})
	if res.StatusCode != 422 || !strings.Contains(body, `id="password-error"`) {
		t.Fatalf("over-long new password: %d, named the new password=%v", res.StatusCode, strings.Contains(body, `id="password-error"`))
	}
	res, body = b.post("/account/password", "/account/password", url.Values{"current": {"wrong current pw"}, "password": {newPassword}})
	if res.StatusCode != 422 || !strings.Contains(body, `class="error"`) {
		t.Fatalf("wrong current password: %d", res.StatusCode)
	}
	res, _ = b.post("/account/password", "/account/password", url.Values{"current": {goodPassword}, "password": {newPassword}})
	if res.StatusCode != 303 {
		t.Fatalf("change: %d", res.StatusCode)
	}
	if res, _ := other.get("/account"); res.StatusCode != 303 {
		t.Errorf("other session still signed in: %d", res.StatusCode)
	}
	if res, _ := b.get("/account"); res.StatusCode != 200 {
		t.Errorf("changing session signed out: %d", res.StatusCode)
	}
	fresh := a.browser(t)
	if res, _ := fresh.signIn(email, goodPassword); res.StatusCode != 422 {
		t.Errorf("old password still accepted: %d", res.StatusCode)
	}
	if res, _ := fresh.signIn(email, newPassword); res.StatusCode != 303 {
		t.Errorf("new password refused: %d", res.StatusCode)
	}
	kinds := waitForMail(t, a, 2)
	if kinds[len(kinds)-1] != "password-changed" {
		t.Errorf("mail kinds %v", kinds)
	}
}

// AC12: five failed sign-ins for an address, or ten from an IP, then 429.
func TestAC12RateLimits(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	email := unique("ac12")
	setup := a.browser(t)
	setup.signUp(email, goodPassword)

	t.Run("sign-up per address", func(t *testing.T) {
		// .claude/rules/api.md: sign-up is limited per account key too, so many
		// IPs cannot flood one address with "you already have an account" mail.
		target := unique("flood")
		for i := 0; i < 10; i++ {
			b := a.browser(t)
			b.ip = fmt.Sprintf("10.7.0.%d", i+1)
			if res, _ := b.post("/sign-up", "/sign-up", url.Values{"email": {target}, "password": {goodPassword}}); res.StatusCode != 200 {
				t.Fatalf("sign-up %d: %d", i+1, res.StatusCode)
			}
		}
		b := a.browser(t)
		b.ip = "10.7.0.99"
		if res, _ := b.post("/sign-up", "/sign-up", url.Values{"email": {target}, "password": {goodPassword}}); res.StatusCode != 429 {
			t.Errorf("eleventh sign-up for one address: %d", res.StatusCode)
		}
	})
	t.Run("per address", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			b := a.browser(t)
			b.ip = fmt.Sprintf("10.1.0.%d", i+1) // a different IP each time
			if res, _ := b.signIn(email, "wrong password here"); res.StatusCode != 422 {
				t.Fatalf("attempt %d: %d", i+1, res.StatusCode)
			}
		}
		b := a.browser(t)
		b.ip = "10.1.0.99"
		res, body := b.signIn(email, goodPassword) // right password, still refused
		if res.StatusCode != 429 || res.Header.Get("Retry-After") == "" || !strings.Contains(body, "Too many") {
			t.Errorf("sixth attempt: %d retry-after=%q", res.StatusCode, res.Header.Get("Retry-After"))
		}
		if b.cookie("__Host-session") != nil {
			t.Error("rate-limited sign-in set a cookie")
		}
	})
	t.Run("concurrent attempts share one count", func(t *testing.T) {
		// Twenty wrong passwords at once for a fresh address: at most five may
		// be checked, the rest must be 429 before any hashing.
		target := unique("race")
		a.browser(t).signUp(target, goodPassword)
		var attempts [20]*browser
		for i := range attempts {
			attempts[i] = a.browser(t)
			attempts[i].ip = fmt.Sprintf("10.5.%d.1", i) // IPs differ, so only the address counts
			attempts[i].get("/sign-in")                  // cookies and token ready before the burst
		}
		var wg sync.WaitGroup
		var checked, limited atomic.Int32
		for _, b := range attempts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, page := b.get("/sign-in")
				token := csrfField.FindStringSubmatch(page)[1]
				res, _ := b.postRaw("/sign-in", url.Values{"email": {target}, "password": {"wrong password here"}, "csrf": {token}}, "Origin", a.URL)
				switch res.StatusCode {
				case 422:
					checked.Add(1)
				case 429:
					limited.Add(1)
				}
			}()
		}
		wg.Wait()
		if checked.Load() != 5 || limited.Load() != 15 {
			t.Errorf("checked %d, limited %d; want 5 and 15", checked.Load(), limited.Load())
		}
	})
	t.Run("per IP", func(t *testing.T) {
		b := a.browser(t)
		b.ip = "10.2.0.1"
		for i := 0; i < 10; i++ {
			if res, _ := b.signIn(unique("ip"), "wrong password here"); res.StatusCode != 422 {
				t.Fatalf("attempt %d: %d", i+1, res.StatusCode)
			}
		}
		if res, _ := b.signIn(unique("ip"), "wrong password here"); res.StatusCode != 429 {
			t.Errorf("eleventh attempt: %d", res.StatusCode)
		}
	})
	t.Run("X-Forwarded-For is ignored without a trusted proxy", func(t *testing.T) {
		plain := newAppWith(t, nil) // no TRUSTED_PROXY: every request is from 127.0.0.1
		setup := plain.browser(t)
		setup.signUp(email, goodPassword)
		for i := 0; i < 10; i++ {
			b := plain.browser(t)
			b.ip = fmt.Sprintf("10.9.0.%d", i+1) // would be a fresh IP if the header were trusted
			if res, _ := b.signIn(unique("xff"), "wrong password here"); res.StatusCode != 422 {
				t.Fatalf("attempt %d: %d", i+1, res.StatusCode)
			}
		}
		b := plain.browser(t)
		b.ip = "10.9.0.99"
		if res, _ := b.signIn(unique("xff"), "wrong password here"); res.StatusCode != 429 {
			t.Errorf("forged X-Forwarded-For escaped the per-IP limit: %d", res.StatusCode)
		}
	})
	t.Run("password change has its own limit", func(t *testing.T) {
		b := a.browser(t)
		b.ip = "10.4.0.1"
		own := unique("pwlimit") // the shared address is already limited above
		b.signUp(own, goodPassword)
		if res, _ := b.signIn(own, goodPassword); res.StatusCode != 303 {
			t.Fatalf("sign-in: %d", res.StatusCode)
		}
		var last int
		for i := 0; i < 6; i++ {
			res, _ := b.post("/account/password", "/account/password", url.Values{"current": {"wrong current pw"}, "password": {"another long password"}})
			last = res.StatusCode
		}
		if last != 429 {
			t.Errorf("sixth wrong current password: %d", last)
		}
	})
	t.Run("sign-up has its own limit", func(t *testing.T) {
		b := a.browser(t)
		b.ip = "10.3.0.1"
		var last int
		for i := 0; i < 11; i++ {
			res, _ := b.post("/sign-up", "/sign-up", url.Values{"email": {unique("su")}, "password": {goodPassword}})
			last = res.StatusCode
		}
		if last != 429 {
			t.Errorf("eleventh sign-up from one IP: %d", last)
		}
	})
}

// AC17: cross-site or token-less posts are 403 and change nothing, signed in
// or not; GET on a write path is 405.
func TestAC17CrossSiteWritesRefused(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	email := unique("ac17")
	b := a.browser(t)
	b.signUp(email, goodPassword)

	t.Run("sign-in without the pre-session token", func(t *testing.T) {
		fresh := a.browser(t)
		res, body := fresh.postRaw("/sign-in", url.Values{"email": {email}, "password": {goodPassword}}, "Origin", a.URL)
		if res.StatusCode != 403 || !strings.Contains(body, "csrf") || fresh.cookie("__Host-session") != nil {
			t.Errorf("got %d %q", res.StatusCode, body)
		}
	})
	t.Run("sign-in with a cross-site origin", func(t *testing.T) {
		fresh := a.browser(t)
		_, page := fresh.get("/sign-in")
		token := csrfField.FindStringSubmatch(page)[1]
		res, _ := fresh.postRaw("/sign-in", url.Values{"email": {email}, "password": {goodPassword}, "csrf": {token}}, "Origin", "https://evil.example")
		if res.StatusCode != 403 || fresh.cookie("__Host-session") != nil {
			t.Errorf("got %d", res.StatusCode)
		}
	})
	if res, _ := b.signIn(email, goodPassword); res.StatusCode != 303 {
		t.Fatalf("sign-in before the signed-in cases: %d", res.StatusCode)
	}
	t.Run("signed-in post with another session's token", func(t *testing.T) {
		other := a.browser(t)
		_, page := other.get("/sign-in")
		token := csrfField.FindStringSubmatch(page)[1]
		res, body := b.postRaw("/account/password", url.Values{"current": {goodPassword}, "password": {"another long password"}, "csrf": {token}}, "Origin", a.URL)
		if res.StatusCode != 403 {
			t.Errorf("got %d %.300s", res.StatusCode, body)
		}
		if res, _ := a.browser(t).signIn(email, goodPassword); res.StatusCode != 303 {
			t.Error("password changed despite the refusal")
		}
	})
	t.Run("signed-in post with Sec-Fetch-Site cross-site", func(t *testing.T) {
		_, page := b.get("/account")
		token := csrfField.FindStringSubmatch(page)[1]
		res, _ := b.postRaw("/sign-out", url.Values{"csrf": {token}}, "Sec-Fetch-Site", "cross-site")
		if res.StatusCode != 403 || b.cookie("__Host-session") == nil {
			t.Errorf("got %d, signed out=%v", res.StatusCode, b.cookie("__Host-session") == nil)
		}
	})
	t.Run("post with a valid token but neither Origin nor Sec-Fetch-Site", func(t *testing.T) {
		_, page := b.get("/account")
		token := csrfField.FindStringSubmatch(page)[1]
		res, _ := b.postRaw("/sign-out", url.Values{"csrf": {token}})
		if res.StatusCode != 403 || b.cookie("__Host-session") == nil {
			t.Errorf("got %d", res.StatusCode)
		}
	})
	t.Run("GET on a write path", func(t *testing.T) {
		res, body := get(t, a, "/sign-out", "Accept", "application/json")
		if p := readProblem(t, res, body); res.StatusCode != 405 || p.Type != "/problems/method-not-allowed" {
			t.Errorf("got %d %+v", res.StatusCode, p)
		}
	})
}

// AC26: after sign-up, a failed sign-in and a page with a report note, the
// structured log holds no address, password, token or note.
func TestAC26NothingPersonalInTheLog(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	id := a.addSpotWith(t, "Logged Spot", "Leeds", "approved", attrs{Wifi: 3, Noise: 2})
	const note = "SECRET-NOTE-TEXT about the window table"
	a.addReport(t, id, attrs{Wifi: 3, Noise: 2}, note, false, 1)
	email := unique("ac26")
	b := a.browser(t)
	b.signUp(email, goodPassword)
	b.signIn(email, "the wrong password!")
	b.signIn(email, goodPassword)
	token := b.cookie("__Host-session").Value
	b.get(fmt.Sprintf("/spots/%d", id))
	b.get("/search?area=" + url.QueryEscape(email)) // personal data in a query string
	const myNote, reason = "MY-OWN-NOTE about the sockets", "MY-REASON this is spam"
	rp := fmt.Sprintf("/spots/%d/reports/new", id)
	b.post(rp, rp, reportForm(myNote))
	pp := fmt.Sprintf("/spots/%d/problems/new", id)
	b.post(pp, pp, url.Values{"reason": {reason}})
	b.post(rp, rp, reportForm(strings.Repeat("x", 501))) // a refused write must not log its content either
	waitForMail(t, a, 1)

	logs := a.logs.String()
	for name, secret := range map[string]string{"email": email, "local part": strings.Split(email, "@")[0], "password": goodPassword, "session token": token, "note": note, "my note": myNote, "reason": reason, "refused note": strings.Repeat("x", 501)} {
		if strings.Contains(logs, secret) {
			t.Errorf("log contains the %s", name)
		}
	}
	if !strings.Contains(logs, `"msg":"request"`) {
		t.Error("log has no request lines; nothing was checked")
	}
}

// AC25 (goroutine clause): the welcome mail is sent in a goroutine, and its
// log line carries the request's correlation ID.
func TestAC25CorrelationIDReachesTheGoroutine(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	b := a.browser(t)
	_, page := b.get("/sign-up")
	token := csrfField.FindStringSubmatch(page)[1]
	req, _ := http.NewRequest(http.MethodPost, a.URL+"/sign-up", strings.NewReader(url.Values{"email": {unique("ac25")}, "password": {goodPassword}, "csrf": {token}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", a.URL)
	req.Header.Set("traceparent", "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab-00f067aa0ba902b7-01")
	res, _ := b.do(req)
	if res.StatusCode != 200 {
		t.Fatalf("sign-up: %d", res.StatusCode)
	}
	waitForMail(t, a, 1)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(a.logs.String(), "\n") {
			if strings.Contains(line, `"msg":"mail sent"`) {
				if !strings.Contains(line, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab") {
					t.Fatalf("mail goroutine lost the correlation id: %s", line)
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no 'mail sent' log line")
}

// AC11: deleting the account with the right password removes the user, its
// sessions, reports, problem reports and pending spot; an approved spot stays
// with no author, a decision it made keeps no name, and the visitor is signed
// out (ADR 0011). A wrong password changes nothing.
func TestAC11DeleteAccount(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	b := signedIn(t, a, "ac11")
	other := a.browser(t)
	other.signIn(b.email, goodPassword)
	ctx := context.Background()
	var userID int64
	if err := a.admin.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, b.email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	approved := a.addSpotWith(t, "Kept Café", "Leeds", "approved", attrs{Wifi: 2, Noise: 2})
	pending := a.addSpotWith(t, "Gone Café", "Leeds", "pending", attrs{Wifi: 2, Noise: 2})
	decided := a.addSpotWith(t, "Decided Café", "Leeds", "approved", attrs{Wifi: 2, Noise: 2})
	if _, err := a.admin.Exec(ctx, `UPDATE spots SET created_by = $1 WHERE id IN ($2, $3)`, userID, approved, pending); err != nil {
		t.Fatal(err)
	}
	if _, err := a.admin.Exec(ctx, `UPDATE spots SET moderated_by = $1, moderated_at = now() WHERE id = $2`, userID, decided); err != nil {
		t.Fatal(err)
	}
	a.addReportBy(t, userID, approved, attrs{Wifi: 4, Noise: 1}, "my report", false, 1)
	if _, err := a.admin.Exec(ctx, `INSERT INTO problem_reports (spot_id, reporter_id, reason) VALUES ($1, $2, 'my problem')`, decided, userID); err != nil {
		t.Fatal(err)
	}
	count := func(query string) int64 {
		t.Helper()
		var n int64
		if err := a.admin.QueryRow(ctx, query, userID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	res, body := b.get("/account")
	if res.StatusCode != 200 || !strings.Contains(body, "/account/delete") {
		t.Fatalf("account page: %d, offers deletion=%v", res.StatusCode, strings.Contains(body, "/account/delete"))
	}
	res, body = b.post("/account/delete", "/account/delete", url.Values{"password": {"not the password"}})
	if res.StatusCode != 422 || !strings.Contains(body, `class="error"`) {
		t.Fatalf("wrong password: %d", res.StatusCode)
	}
	if n := count(`SELECT count(*) FROM users WHERE id = $1`); n != 1 {
		t.Fatalf("wrong password deleted the account")
	}

	res, _ = b.post("/account/delete", "/account/delete", url.Values{"password": {goodPassword}})
	if res.StatusCode != 303 || res.Header.Get("Location") != "/account/deleted" {
		t.Fatalf("delete: %d to %q", res.StatusCode, res.Header.Get("Location"))
	}
	res, body = b.get("/account/deleted")
	if res.StatusCode != 200 || !strings.Contains(body, "Your account has been deleted") || strings.Contains(body, b.email) {
		t.Errorf("confirmation page: %d, shows address=%v", res.StatusCode, strings.Contains(body, b.email))
	}

	for query, want := range map[string]int64{
		`SELECT count(*) FROM users WHERE id = $1`:                                                                          0,
		`SELECT count(*) FROM sessions WHERE user_id = $1`:                                                                  0,
		`SELECT count(*) FROM reports WHERE author_id = $1`:                                                                 0,
		`SELECT count(*) FROM problem_reports WHERE reporter_id = $1`:                                                       0,
		`SELECT count(*) FROM spots WHERE created_by = $1 OR moderated_by = $1`:                                             0,
		`SELECT count(*) FROM spots WHERE name IN ('Kept Café', 'Decided Café') AND created_by IS NULL AND $1::bigint <> 0`: 2,
		`SELECT count(*) FROM spots WHERE name = 'Gone Café' AND $1::bigint <> 0`:                                           0,
	} {
		if n := count(query); n != want {
			t.Errorf("%s: %d, want %d", query, n, want)
		}
	}
	if b.cookie("__Host-session") != nil {
		t.Error("session cookie not cleared")
	}
	for name, br := range map[string]*browser{"deleting": b, "other": other} {
		if res, _ := br.get("/account"); res.StatusCode != 303 {
			t.Errorf("%s session still signed in: %d", name, res.StatusCode)
		}
	}
	if res, _ := a.browser(t).signIn(b.email, goodPassword); res.StatusCode != 422 {
		t.Errorf("deleted account can sign in: %d", res.StatusCode)
	}
	if kinds := waitForMail(t, a, 2); kinds[len(kinds)-1] != "account-deleted" {
		t.Errorf("mail kinds %v", kinds)
	}
	if _, body := staff(t, a, "ac11mod", "moderator").get("/moderation"); !strings.Contains(body, "a deleted account") {
		t.Error("the queue's history names a deleted moderator")
	}
	if strings.Contains(a.logs.String(), b.email) {
		t.Error("the address reached the log")
	}
}

// Behind a proxy whose address is not TRUSTED_PROXY every client shares the
// peer's address and so one rate-limit bucket; the misconfiguration is logged
// once, without the address (security audit, slice 6).
func TestUntrustedPeerIsLoggedOnce(t *testing.T) {
	t.Parallel()
	a := newAppWith(t, map[string]string{"TRUSTED_PROXY": "10.9.9.9"})
	b := a.browser(t)
	for i := 0; i < 2; i++ {
		b.signIn(unique("peer"), goodPassword)
	}
	if n := strings.Count(a.logs.String(), "other than TRUSTED_PROXY"); n != 1 {
		t.Errorf("untrusted peer logged %d times, want once", n)
	}
	if strings.Contains(a.logs.String(), "127.0.0.1") {
		t.Error("the peer address reached the log")
	}
}

// AC11 under a race (whole-codebase review, MUST-FIX): a moderator approves the
// member's pending spot while the member deletes their account. The spot was
// approved, so it stays, anonymised; it must not vanish because the deletion
// decided on a snapshot from before the approval.
func TestAC11DeleteKeepsASpotApprovedMeanwhile(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	b := signedIn(t, a, "ac11race")
	ctx := context.Background()
	var userID int64
	if err := a.admin.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, b.email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	spot := a.addSpotWith(t, "Racing Café", "Leeds", "pending", attrs{Wifi: 2, Noise: 2})
	if _, err := a.admin.Exec(ctx, `UPDATE spots SET created_by = $1 WHERE id = $2`, userID, spot); err != nil {
		t.Fatal(err)
	}
	tx, err := a.admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE spots SET status = 'approved', moderated_at = now() WHERE id = $1`, spot); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() { // blocks on the row the approval holds until the commit below
		res, _ := b.post("/account/delete", "/account/delete", url.Values{"password": {goodPassword}})
		done <- res.StatusCode
	}()
	time.Sleep(300 * time.Millisecond)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if status := <-done; status != 303 {
		t.Fatalf("delete: %d", status)
	}
	var kept bool
	if err := a.admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM spots WHERE id = $1 AND status = 'approved' AND created_by IS NULL)`, spot).Scan(&kept); err != nil {
		t.Fatal(err)
	}
	if !kept {
		t.Error("the spot approved during the deletion is gone")
	}
}

// Bytes Postgres refuses (invalid UTF-8, NUL) in any field are a 422 or are
// cleaned, never a 500 (whole-codebase review).
func TestInvalidBytesNeverReachPostgres(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	b := signedIn(t, a, "bytes")
	form := spotForm("Byte Caf\xff")
	form.Set("area", "Le\xffeds\x00")
	if res, _ := b.post("/spots/new", "/spots/new", form); res.StatusCode != 303 {
		t.Errorf("spot with invalid bytes: %d, want 303 after cleaning", res.StatusCode)
	}
	if res, _ := a.browser(t).signIn("ba\xffd@example.test", goodPassword); res.StatusCode != 422 {
		t.Errorf("sign-in with invalid bytes: %d, want 422", res.StatusCode)
	}
}

package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func spotForm(name string) url.Values {
	return url.Values{"name": {name}, "kind": {"cafe"}, "area": {"Leeds, LS6"}, "address": {"1 Test Street"},
		"opening_hours": {"Mon-Fri 8-18"}, "wifi": {"3"}, "noise": {"2"}, "power": {"1"}, "step_free": {"1"}}
}

func reportForm(note string) url.Values {
	return url.Values{"wifi": {"4"}, "noise": {"1"}, "power": {"1"}, "time_limit": {"0"}, "step_free": {"1"},
		"accessible_toilet": {"0"}, "quiet_space": {"1"}, "note": {note}}
}

// signedIn signs up and in a fresh member and returns the browser.
func signedIn(t *testing.T, a *app, prefix string) *browser {
	t.Helper()
	b := a.browser(t)
	email := unique(prefix)
	b.signUp(email, goodPassword)
	if res, _ := b.signIn(email, goodPassword); res.StatusCode != 303 {
		t.Fatalf("sign-in: %d", res.StatusCode)
	}
	return b
}

// AC13: contributing needs an account: GET redirects to sign in with a safe
// return path, POST is a 401 problem.
func TestAC13ContributingNeedsAnAccount(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	id := a.addSpotWith(t, "Open Café", "Leeds", "approved", attrs{Wifi: 3, Noise: 2})
	visitor := a.browser(t)
	_, page := visitor.get("/sign-in")
	token := csrfField.FindStringSubmatch(page)[1]
	for _, path := range []string{"/spots/new", fmt.Sprintf("/spots/%d/reports/new", id), fmt.Sprintf("/spots/%d/problems/new", id)} {
		res, _ := visitor.get(path)
		if res.StatusCode != 303 || res.Header.Get("Location") != "/sign-in?next="+url.QueryEscape(path) {
			t.Errorf("GET %s: %d -> %q", path, res.StatusCode, res.Header.Get("Location"))
		}
		// With a valid pre-session token, so it is the sign-in check that answers, not CSRF.
		form := spotForm("Sneaky")
		form.Set("csrf", token)
		form.Set("reason", "sneaky")
		res, body := visitor.postRaw(path, form, "Origin", a.URL, "Accept", "application/json")
		if p := readProblem(t, res, body); res.StatusCode != 401 || p.Type != "/problems/sign-in-required" {
			t.Errorf("POST %s: %d %+v", path, res.StatusCode, p)
		}
	}
	var n int
	_ = a.admin.QueryRow(context.Background(), `SELECT count(*) FROM spots WHERE name = 'Sneaky'`).Scan(&n)
	var m int
	_ = a.admin.QueryRow(context.Background(), `SELECT count(*) FROM reports WHERE spot_id = $1`, id).Scan(&m)
	var k int
	_ = a.admin.QueryRow(context.Background(), `SELECT count(*) FROM problem_reports`).Scan(&k)
	if n+m+k != 0 {
		t.Errorf("a signed-out POST wrote rows: spots %d, reports %d, problems %d", n, m, k)
	}
}

// AC14 and AC6: a new spot is pending, shown to its author as awaiting
// approval, and invisible to search and to everyone else.
func TestAC14NewSpotIsPending(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	author := signedIn(t, a, "author")

	res, _ := author.post("/spots/new", "/spots/new", spotForm("Brand New Café"))
	if res.StatusCode != 303 || !strings.HasPrefix(res.Header.Get("Location"), "/spots/") {
		t.Fatalf("add spot: %d -> %q", res.StatusCode, res.Header.Get("Location"))
	}
	spotPath := res.Header.Get("Location")
	var status string
	var createdBy int64
	if err := a.admin.QueryRow(context.Background(), `SELECT status, created_by FROM spots WHERE name = 'Brand New Café'`).Scan(&status, &createdBy); err != nil || status != "pending" || createdBy == 0 {
		t.Fatalf("row: %q by %d, %v", status, createdBy, err)
	}
	res, body := author.get(spotPath)
	if res.StatusCode != 200 {
		t.Fatalf("author's view: %d", res.StatusCode)
	}
	has(t, body, "Brand New Café", "waiting approval", "Leeds, LS6", "Mon-Fri 8-18")
	if _, body := author.get("/account"); !strings.Contains(body, "Brand New Café") || !strings.Contains(body, "pending") {
		t.Error("account page does not list the pending spot with its status")
	}

	_, body = get(t, a, "/search?area=leeds")
	lacks(t, body, "Brand New Café")
	visitor := a.browser(t)
	if res, _ := visitor.get(spotPath); res.StatusCode != 404 {
		t.Errorf("visitor sees a pending spot: %d", res.StatusCode)
	}
	other := signedIn(t, a, "other")
	if res, _ := other.get(spotPath); res.StatusCode != 404 {
		t.Errorf("another member sees a pending spot: %d", res.StatusCode)
	}
}

// AC15: validation keeps what was typed and names each problem.
func TestAC15ValidationKeepsWhatWasTyped(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	member := signedIn(t, a, "ac15")
	form := spotForm("")
	form.Set("area", strings.Repeat("a", 81))
	form.Set("address", "Kept Address 12")
	res, body := member.post("/spots/new", "/spots/new", form)
	if res.StatusCode != 422 {
		t.Fatalf("status %d", res.StatusCode)
	}
	has(t, body, `role="alert"`, `href="#name"`, `href="#area"`, `id="name-error"`, `id="area-error"`, `value="Kept Address 12"`, `value="`+strings.Repeat("a", 81)+`"`)
	if strings.Count(body, "checked") < 2 {
		t.Error("ticked attributes were not kept")
	}
	var n int
	_ = a.admin.QueryRow(context.Background(), `SELECT count(*) FROM spots WHERE address = 'Kept Address 12'`).Scan(&n)
	if n != 0 {
		t.Error("an invalid spot was saved")
	}
	// An unknown kind or an out-of-range rating is refused too.
	form = spotForm("Odd Kind")
	form.Set("kind", "casino")
	form.Set("wifi", "9")
	res, body = member.post("/spots/new", "/spots/new", form)
	if res.StatusCode != 422 {
		t.Errorf("unknown kind: %d", res.StatusCode)
	}
	// The summary's link to a rating group must land on it, and the group must name its error.
	has(t, body, `href="#wifi"`, `<fieldset id="wifi" aria-describedby="wifi-error"`, `id="wifi-error"`, `<fieldset id="noise"`)
}

// AC16: a report shows at once with today's date and moves the attributes.
func TestAC16ReportShowsAtOnce(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	id := a.addSpotWith(t, "Reported Café", "Leeds", "approved", attrs{Wifi: 1, Noise: 3})
	member := signedIn(t, a, "ac16")
	path := fmt.Sprintf("/spots/%d/reports/new", id)

	res, _ := member.post(path, path, reportForm("Lovely quiet corner upstairs."))
	if res.StatusCode != 303 || res.Header.Get("Location") != fmt.Sprintf("/spots/%d", id) {
		t.Fatalf("report: %d -> %q", res.StatusCode, res.Header.Get("Location"))
	}
	_, body := member.get(fmt.Sprintf("/spots/%d", id))
	today := time.Now().UTC().Format("2006-01-02")
	has(t, body, "Lovely quiet corner upstairs.", `<time datetime="`+today+`"`, "From 1 report", "<dd>Excellent</dd>", "<dd>Quiet</dd>")

	res, body = member.post(path, path, reportForm(strings.Repeat("n", 501)))
	if res.StatusCode != 422 || !strings.Contains(body, "500") {
		t.Errorf("long note: %d", res.StatusCode)
	}
	// A report on a pending or unknown spot is a 404 problem, for its author too.
	author := signedIn(t, a, "pendingauthor")
	res, _ = author.post("/spots/new", "/spots/new", spotForm("Author's Pending"))
	p := res.Header.Get("Location") + "/reports/new"
	if res, _ := author.get(p); res.StatusCode != 404 {
		t.Errorf("report form for one's own pending spot: %d", res.StatusCode)
	}
	if res, _ := author.post("/spots/new", p, reportForm("x")); res.StatusCode != 404 {
		t.Errorf("report on a pending spot: %d", res.StatusCode)
	}
}

// AC16, one vote per member: three members' reports decide an attribute, and a
// fourth member posting five contrary reports does not overturn them. The
// earlier reports stay on the page as history.
func TestAC16OneVotePerMemberPerSpot(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	id := a.addSpotWith(t, "Contested Café", "Leeds", "approved", attrs{Wifi: 3, Noise: 2, StepFree: true})
	for i := 0; i < 3; i++ {
		a.addReportBy(t, a.seedMember(t), id, attrs{Wifi: 3, Noise: 2, StepFree: false}, "No step-free entrance", false, 3-i)
	}
	_, body := get(t, a, fmt.Sprintf("/spots/%d", id))
	has(t, body, "<dt>Step-free entrance</dt><dd>No</dd>", "From 3 reports")

	loud := signedIn(t, a, "loud")
	path := fmt.Sprintf("/spots/%d/reports/new", id)
	for i := 0; i < 5; i++ {
		form := reportForm(fmt.Sprintf("Insisting %d", i))
		form.Set("step_free", "1")
		if res, _ := loud.post(path, path, form); res.StatusCode != 303 {
			t.Fatalf("report %d: %d", i, res.StatusCode)
		}
	}
	_, body = get(t, a, fmt.Sprintf("/spots/%d", id))
	has(t, body, "<dt>Step-free entrance</dt><dd>No</dd>", "From 8 reports", "Insisting 0", "Insisting 4")
}

// Contributions are rate limited per member, so one account cannot flood the
// queue or set a spot's attributes alone.
func TestContributionsAreLimitedPerMember(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	member := signedIn(t, a, "flood")
	var last int
	for i := 0; i < 11; i++ {
		res, _ := member.post("/spots/new", "/spots/new", spotForm(fmt.Sprintf("Flood %d", i)))
		last = res.StatusCode
	}
	if last != 429 {
		t.Errorf("eleventh spot in a quarter hour: %d", last)
	}
	id := a.addSpotWith(t, "Flooded Café", "Leeds", "approved", attrs{Wifi: 3, Noise: 2})
	path := fmt.Sprintf("/spots/%d/reports/new", id)
	for i := 0; i < 20; i++ {
		member.post(path, path, reportForm(""))
	}
	if res, _ := member.post(path, path, reportForm("")); res.StatusCode != 429 {
		t.Errorf("twenty-first report in a quarter hour: %d", res.StatusCode)
	}
}

// AC22: a problem report on a spot or a report lands in the queue, open.
func TestAC22ReportAProblem(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	id := a.addSpotWith(t, "Problem Café", "Leeds", "approved", attrs{Wifi: 3, Noise: 2})
	a.addReport(t, id, attrs{Wifi: 3, Noise: 2}, "Spammy note", false, 1)
	var reportID int64
	_ = a.admin.QueryRow(context.Background(), `SELECT id FROM reports WHERE spot_id = $1`, id).Scan(&reportID)
	member := signedIn(t, a, "ac22")

	path := fmt.Sprintf("/spots/%d/problems/new", id)
	res, body := member.post(path, path, url.Values{"reason": {"This café closed last year."}})
	if res.StatusCode != 200 || !strings.Contains(body, "Thank you") {
		t.Fatalf("spot problem: %d", res.StatusCode)
	}
	path = fmt.Sprintf("/reports/%d/problems/new", reportID)
	res, body = member.post(path, path, url.Values{"reason": {"Advertising, not a report."}})
	if res.StatusCode != 200 || !strings.Contains(body, "Thank you") {
		t.Fatalf("report problem: %d", res.StatusCode)
	}
	var open int
	_ = a.admin.QueryRow(context.Background(), `SELECT count(*) FROM problem_reports WHERE status = 'open'
		AND ((spot_id = $1 AND report_id IS NULL) OR (report_id = $2 AND spot_id IS NULL))`, id, reportID).Scan(&open)
	if open != 2 {
		t.Errorf("%d open problem reports, want 2", open)
	}
	if res, _ := member.post(path, path, url.Values{"reason": {""}}); res.StatusCode != 422 {
		t.Errorf("empty reason: %d", res.StatusCode)
	}
	if res, _ := member.post("/spots/new", "/reports/999999/problems/new", url.Values{"reason": {"x"}}); res.StatusCode != 404 {
		t.Errorf("unknown report: %d", res.StatusCode)
	}
	// Nothing invisible can be flagged: a hidden report, a pending spot (even by its author).
	a.addReport(t, id, attrs{Wifi: 3, Noise: 2}, "Hidden note", true, 0)
	var hidden int64
	_ = a.admin.QueryRow(context.Background(), `SELECT id FROM reports WHERE hidden`).Scan(&hidden)
	hp := fmt.Sprintf("/reports/%d/problems/new", hidden)
	if res, _ := member.get(hp); res.StatusCode != 404 {
		t.Errorf("problem form for a hidden report: %d", res.StatusCode)
	}
	if res, _ := member.post("/spots/new", hp, url.Values{"reason": {"x"}}); res.StatusCode != 404 {
		t.Errorf("problem on a hidden report: %d", res.StatusCode)
	}
	res, _ = member.post("/spots/new", "/spots/new", spotForm("My Pending"))
	pp := res.Header.Get("Location") + "/problems/new"
	if res, _ := member.get(pp); res.StatusCode != 404 {
		t.Errorf("problem form for one's own pending spot: %d", res.StatusCode)
	}
	if res, _ := member.post("/spots/new", pp, url.Values{"reason": {"x"}}); res.StatusCode != 404 {
		t.Errorf("problem on a pending spot: %d", res.StatusCode)
	}
	_ = a.admin.QueryRow(context.Background(), `SELECT count(*) FROM problem_reports`).Scan(&open)
	if open != 2 {
		t.Errorf("%d problem reports after the refused ones, want 2", open)
	}
}

var _ = http.StatusOK

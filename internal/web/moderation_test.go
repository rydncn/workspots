package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// staff signs in a fresh member and gives them role directly in the database.
func staff(t *testing.T, a *app, prefix, role string) *browser {
	t.Helper()
	b := signedIn(t, a, prefix)
	a.setRole(t, b.email, role)
	return b
}

func (a *app) setRole(t *testing.T, email, role string) {
	t.Helper()
	if _, err := a.admin.Exec(context.Background(), `UPDATE users SET role = $2 WHERE email = $1`, email, role); err != nil {
		t.Fatal(err)
	}
}

func (a *app) role(t *testing.T, email string) string {
	t.Helper()
	var role string
	if err := a.admin.QueryRow(context.Background(), `SELECT role FROM users WHERE email = $1`, email).Scan(&role); err != nil {
		t.Fatal(err)
	}
	return role
}

// AC18: moderation is for moderators. A visitor gets a 401 problem, a member
// a 403, on the page and on every action; nothing changes.
func TestAC18ModerationIsForModerators(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	spot := a.addSpotWith(t, "Pending Café", "Leeds", "pending", attrs{Wifi: 3, Noise: 2})
	open := a.addSpotWith(t, "Open Café", "Leeds", "approved", attrs{Wifi: 3, Noise: 2})
	a.addReport(t, open, attrs{Wifi: 3, Noise: 2}, "A note", false, 1)
	var report int64
	_ = a.admin.QueryRow(context.Background(), `SELECT id FROM reports WHERE spot_id = $1`, open).Scan(&report)
	var problem int64
	_ = a.admin.QueryRow(context.Background(), `INSERT INTO problem_reports (report_id, reporter_id, reason) VALUES ($1, $2, 'spam') RETURNING id`, report, a.seeder).Scan(&problem)
	actions := []string{
		fmt.Sprintf("/moderation/spots/%d/approve", spot), fmt.Sprintf("/moderation/spots/%d/reject", spot),
		fmt.Sprintf("/moderation/reports/%d/hide", report), fmt.Sprintf("/moderation/problems/%d/resolve", problem),
	}
	for _, who := range []struct {
		name string
		b    *browser
		want int
		typ  string
	}{
		{"visitor", a.browser(t), 401, "/problems/sign-in-required"},
		{"member", signedIn(t, a, "member"), 403, "/problems/forbidden"},
	} {
		res, _ := who.b.get("/moderation")
		if res.StatusCode != who.want {
			t.Errorf("%s GET /moderation: %d", who.name, res.StatusCode)
		}
		_, page := who.b.get("/sign-in")
		token := csrfField.FindStringSubmatch(page)[1] // valid for this browser, so the role check answers, not CSRF
		for _, path := range actions {
			res, body := who.b.postRaw(path, url.Values{"csrf": {token}, "reason": {"because"}}, "Origin", a.URL, "Accept", "application/json")
			if p := readProblem(t, res, body); res.StatusCode != who.want || p.Type != who.typ {
				t.Errorf("%s POST %s: %d %+v", who.name, path, res.StatusCode, p)
			}
		}
	}
	var changed int
	_ = a.admin.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM spots WHERE status <> 'pending' AND id = $1)
		+ (SELECT count(*) FROM reports WHERE hidden) + (SELECT count(*) FROM problem_reports WHERE status <> 'open')`, spot).Scan(&changed)
	if changed != 0 {
		t.Errorf("a refused action changed %d rows", changed)
	}
	if res, body := staff(t, a, "mod", "moderator").get("/moderation"); res.StatusCode != 200 || !strings.Contains(body, "Pending Café") {
		t.Errorf("moderator's queue: %d", res.StatusCode)
	}
}

// AC19 (and AC6's moderator exception): approving puts a spot in search;
// rejecting keeps it out and tells its author.
func TestAC19ApproveOrReject(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	author := signedIn(t, a, "author")
	res, _ := author.post("/spots/new", "/spots/new", spotForm("Approve Me Café"))
	approvePath := res.Header.Get("Location")
	res, _ = author.post("/spots/new", "/spots/new", spotForm("Reject Me Café"))
	rejectPath := res.Header.Get("Location")
	mod := staff(t, a, "mod", "moderator")

	if res, _ := mod.get(approvePath); res.StatusCode != 200 {
		t.Errorf("moderator's view of a pending spot: %d (AC6)", res.StatusCode)
	}
	_, queue := mod.get("/moderation")
	has(t, queue, "Approve Me Café", "Reject Me Café", `action="/moderation`+approvePath+`/approve"`)
	if res, _ := mod.post("/moderation", "/moderation"+approvePath+"/approve", url.Values{}); res.StatusCode != 303 || res.Header.Get("Location") != "/moderation" {
		t.Fatalf("approve: %d -> %q", res.StatusCode, res.Header.Get("Location"))
	}
	if res, _ := mod.post("/moderation", "/moderation"+rejectPath+"/reject", url.Values{}); res.StatusCode != 303 {
		t.Fatalf("reject: %d", res.StatusCode)
	}
	_, body := get(t, a, "/search?area=leeds")
	has(t, body, "Approve Me Café")
	lacks(t, body, "Reject Me Café")
	_, queue = mod.get("/moderation")
	lacks(t, queue, "Approve Me Café", "Reject Me Café")
	if _, body := author.get("/account"); !strings.Contains(body, "rejected") {
		t.Error("author's account page does not say rejected")
	}
	if res, body := author.get(rejectPath); res.StatusCode != 200 || !strings.Contains(body, "rejected") {
		t.Errorf("author's view of the rejected spot: %d, says rejected: %v", res.StatusCode, strings.Contains(body, "rejected"))
	}
	if res, _ := a.browser(t).get(rejectPath); res.StatusCode != 404 {
		t.Errorf("visitor sees a rejected spot: %d", res.StatusCode)
	}
	if res, _ := mod.post("/moderation", "/moderation/spots/999999/approve", url.Values{}); res.StatusCode != 404 {
		t.Errorf("approving an unknown spot: %d", res.StatusCode)
	}
}

// AC20: a hidden report leaves the spot page and the spot's attributes; the
// queue shows it hidden with the reason.
func TestAC20HideAReport(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	spot := a.addSpotWith(t, "Hide Café", "Leeds", "approved", attrs{Wifi: 1, Noise: 2})
	a.addReport(t, spot, attrs{Wifi: 2, Noise: 2}, "Honest note", false, 2)
	a.addReportBy(t, a.seedMember(t), spot, attrs{Wifi: 4, Noise: 2}, "SPAMMY NOTE", false, 1)
	var report int64
	_ = a.admin.QueryRow(context.Background(), `SELECT id FROM reports WHERE note = 'SPAMMY NOTE'`).Scan(&report)
	flagger := signedIn(t, a, "flagger")
	flagPath := fmt.Sprintf("/reports/%d/problems/new", report)
	if res, _ := flagger.post(flagPath, flagPath, url.Values{"reason": {"Advertising."}}); res.StatusCode != 200 {
		t.Fatalf("flag: %d", res.StatusCode)
	}
	_, page := get(t, a, fmt.Sprintf("/spots/%d", spot))
	has(t, page, "SPAMMY NOTE", "<dd>Good</dd>") // median of usable (2) and excellent (4) is 3, good

	mod := staff(t, a, "mod", "moderator")
	_, queue := mod.get("/moderation")
	has(t, queue, "SPAMMY NOTE", "Advertising.", "Hide Café")
	lacks(t, queue, "Hidden")
	hidePath := fmt.Sprintf("/moderation/reports/%d/hide", report)
	if res, _ := mod.post("/moderation", hidePath, url.Values{"reason": {""}}); res.StatusCode != 422 {
		t.Errorf("hiding without a reason: %d", res.StatusCode)
	}
	if res, _ := mod.post("/moderation", hidePath, url.Values{"reason": {"Advertising, not a report."}}); res.StatusCode != 303 {
		t.Fatalf("hide: %d", res.StatusCode)
	}
	_, page = get(t, a, fmt.Sprintf("/spots/%d", spot))
	lacks(t, page, "SPAMMY NOTE")
	has(t, page, "Honest note", "<dd>Usable</dd>", "From 1 report")
	_, queue = mod.get("/moderation")
	has(t, queue, "Hidden", "Advertising, not a report.")
	var reason string
	var hiddenAt *string
	_ = a.admin.QueryRow(context.Background(), `SELECT hidden_reason, hidden_at::text FROM reports WHERE id = $1 AND hidden`, report).Scan(&reason, &hiddenAt)
	if reason != "Advertising, not a report." || hiddenAt == nil {
		t.Errorf("row: reason %q, hidden_at %v", reason, hiddenAt)
	}
	if res, _ := mod.post("/moderation", hidePath, url.Values{"reason": {"again"}}); res.StatusCode != 404 {
		t.Errorf("hiding a hidden report: %d", res.StatusCode)
	}
	// Resolving the problem takes it off the queue.
	var problem int64
	_ = a.admin.QueryRow(context.Background(), `SELECT id FROM problem_reports WHERE report_id = $1`, report).Scan(&problem)
	if res, _ := mod.post("/moderation", fmt.Sprintf("/moderation/problems/%d/resolve", problem), url.Values{}); res.StatusCode != 303 {
		t.Fatalf("resolve: %d", res.StatusCode)
	}
	_, queue = mod.get("/moderation")
	lacks(t, queue, "Advertising, not a report.")
}

// AC21: only an admin changes roles.
func TestAC21OnlyAnAdminChangesRoles(t *testing.T) {
	t.Parallel()
	a := newApp(t)
	member := signedIn(t, a, "member")
	mod := staff(t, a, "mod", "moderator")
	admin := staff(t, a, "admin", "admin")

	for _, who := range []struct {
		name string
		b    *browser
		want int
	}{{"visitor", a.browser(t), 401}, {"member", member, 403}, {"moderator", mod, 403}} {
		if res, _ := who.b.get("/admin/users"); res.StatusCode != who.want {
			t.Errorf("%s GET /admin/users: %d", who.name, res.StatusCode)
		}
		_, page := who.b.get("/sign-in")
		token := csrfField.FindStringSubmatch(page)[1]
		res, body := who.b.postRaw("/admin/users", url.Values{"csrf": {token}, "email": {member.email}, "role": {"admin"}}, "Origin", a.URL, "Accept", "application/json")
		if res.StatusCode != who.want {
			t.Errorf("%s POST /admin/users: %d %s", who.name, res.StatusCode, body)
		}
	}
	if got := a.role(t, member.email); got != "member" {
		t.Fatalf("role after refused changes: %q", got)
	}
	if res, _ := member.get("/moderation"); res.StatusCode != 403 {
		t.Fatalf("member before promotion: %d", res.StatusCode)
	}
	res, body := admin.post("/admin/users", "/admin/users", url.Values{"email": {member.email}, "role": {"moderator"}})
	if res.StatusCode != 200 || !strings.Contains(body, member.email) || !strings.Contains(body, "moderator") {
		t.Fatalf("promote: %d", res.StatusCode)
	}
	if got := a.role(t, member.email); got != "moderator" {
		t.Errorf("role after promotion: %q", got)
	}
	if res, _ := member.get("/moderation"); res.StatusCode != 200 {
		t.Errorf("new moderator opens /moderation: %d", res.StatusCode)
	}
	if res, _ := admin.post("/admin/users", "/admin/users", url.Values{"email": {member.email}, "role": {"owner"}}); res.StatusCode != 422 {
		t.Errorf("unknown role: %d", res.StatusCode)
	}
	if res, body := admin.post("/admin/users", "/admin/users", url.Values{"email": {"nobody@example.test"}, "role": {"admin"}}); res.StatusCode != 422 || !strings.Contains(body, "No account") {
		t.Errorf("unknown address: %d", res.StatusCode)
	}
}

var _ = http.StatusOK

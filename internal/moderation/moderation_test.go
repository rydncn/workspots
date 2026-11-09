package moderation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rydncn/workspots/internal/accounts"
	"github.com/rydncn/workspots/internal/moderation"
	"github.com/rydncn/workspots/internal/testdb"
)

// The domain refuses a member whatever the handler did (.claude/rules/api.md).
func TestActionsNeedAModerator(t *testing.T) {
	t.Parallel()
	adminURL, appURL := testdb.New(t)
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	db, err := pgxpool.New(ctx, appURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var member, mod, spot int64
	if err := admin.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('m@example.test', 'x') RETURNING id`).Scan(&member); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, `INSERT INTO users (email, password_hash, role) VALUES ('mod@example.test', 'x', 'moderator') RETURNING id`).Scan(&mod); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow(ctx, `INSERT INTO spots (name, kind, area, address) VALUES ('Pending', 'cafe', 'Leeds', '1 St') RETURNING id`).Scan(&spot); err != nil {
		t.Fatal(err)
	}
	for name, act := range map[string]func(actor int64) error{
		"SetStatus":  func(a int64) error { return moderation.SetStatus(ctx, db, a, spot, "approved") },
		"HideReport": func(a int64) error { return moderation.HideReport(ctx, db, a, 1, "why") },
		"Resolve":    func(a int64) error { return moderation.Resolve(ctx, db, a, 1) },
		"SetRole":    func(a int64) error { _, err := accounts.SetRole(ctx, db, a, "m@example.test", "admin"); return err },
	} {
		if err := act(member); !errors.Is(err, accounts.ErrPermissionDenied) {
			t.Errorf("%s by a member: %v", name, err)
		}
		if err := act(mod); errors.Is(err, accounts.ErrPermissionDenied) == (name != "SetRole") {
			t.Errorf("%s by a moderator: %v", name, err)
		}
	}
	var status string
	_ = admin.QueryRow(ctx, `SELECT status FROM spots WHERE id = $1`, spot).Scan(&status)
	if status != "approved" {
		t.Errorf("the moderator's approval did not land: %q", status)
	}
}

// Rule 19: every state and every action of each moderation lifecycle has a
// decided outcome (docs/design.md, Lifecycles). A state or action without a
// row here fails.
func TestLifecycleTransitions(t *testing.T) {
	t.Parallel()
	adminURL, appURL := testdb.New(t)
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	db, err := pgxpool.New(ctx, appURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var mod, author int64
	_ = admin.QueryRow(ctx, `INSERT INTO users (email, password_hash, role) VALUES ('mod@example.test', 'x', 'moderator') RETURNING id`).Scan(&mod)
	_ = admin.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('a@example.test', 'x') RETURNING id`).Scan(&author)

	// to is the state after the action, or "" when the move is refused with ErrNothingToModerate.
	table := map[string]map[string]map[string]string{
		"spot": { // status
			"pending":  {"approve": "approved", "reject": "rejected"},
			"approved": {"approve": "", "reject": "rejected"},
			"rejected": {"approve": "approved", "reject": ""},
		},
		"report": { // hidden
			"visible": {"hide": "hidden"},
			"hidden":  {"hide": ""},
		},
		"problem": { // status
			"open":     {"resolve": "resolved"},
			"resolved": {"resolve": ""},
		},
	}
	// put makes a row in the state and returns a function reading its state back.
	put := func(lifecycle, state string) (int64, func() string) {
		var id int64
		var read string
		switch lifecycle {
		case "spot":
			_ = admin.QueryRow(ctx, `INSERT INTO spots (name, kind, area, address, status) VALUES ('S', 'cafe', 'Leeds', '1 St', $1) RETURNING id`, state).Scan(&id)
			read = `SELECT status FROM spots WHERE id = $1`
		case "report":
			var spot int64
			_ = admin.QueryRow(ctx, `INSERT INTO spots (name, kind, area, address, status) VALUES ('S', 'cafe', 'Leeds', '1 St', 'approved') RETURNING id`).Scan(&spot)
			_ = admin.QueryRow(ctx, `INSERT INTO reports (spot_id, author_id, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space, hidden)
				VALUES ($1, $2, 2, 2, false, false, false, false, false, $3) RETURNING id`, spot, author, state == "hidden").Scan(&id)
			read = `SELECT CASE WHEN hidden THEN 'hidden' ELSE 'visible' END FROM reports WHERE id = $1`
		case "problem":
			var spot int64
			_ = admin.QueryRow(ctx, `INSERT INTO spots (name, kind, area, address, status) VALUES ('S', 'cafe', 'Leeds', '1 St', 'approved') RETURNING id`).Scan(&spot)
			_ = admin.QueryRow(ctx, `INSERT INTO problem_reports (spot_id, reporter_id, reason, status) VALUES ($1, $2, 'r', $3) RETURNING id`, spot, author, state).Scan(&id)
			read = `SELECT status FROM problem_reports WHERE id = $1`
		}
		return id, func() string {
			var s string
			_ = admin.QueryRow(ctx, read, id).Scan(&s)
			return s
		}
	}
	act := func(action string, id int64) error {
		switch action {
		case "approve":
			return moderation.SetStatus(ctx, db, mod, id, "approved")
		case "reject":
			return moderation.SetStatus(ctx, db, mod, id, "rejected")
		case "hide":
			return moderation.HideReport(ctx, db, mod, id, "why")
		case "resolve":
			return moderation.Resolve(ctx, db, mod, id)
		}
		t.Fatalf("no such action %q", action)
		return nil
	}
	for lifecycle, states := range table {
		for from, actions := range states {
			for action, to := range actions {
				id, state := put(lifecycle, from)
				err := act(action, id)
				switch {
				case to == "" && !errors.Is(err, moderation.ErrNothingToModerate):
					t.Errorf("%s %s --%s--> expected refusal, got %v (now %s)", lifecycle, from, action, err, state())
				case to != "" && (err != nil || state() != to):
					t.Errorf("%s %s --%s--> %s: err %v, now %s", lifecycle, from, action, to, err, state())
				}
			}
		}
	}
}

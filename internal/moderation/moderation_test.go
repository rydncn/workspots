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

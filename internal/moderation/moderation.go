// Package moderation is the queue: problem reports from members (slice 4),
// and approving, rejecting and hiding (slice 5).
package moderation

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rydncn/workspots/internal/accounts"
)

var (
	// ErrTargetNotFound is returned when the spot or report to flag is not visible.
	ErrTargetNotFound = errors.New("there is nothing to report at this address")
	// ErrNothingToModerate is returned when what a moderator acts on is not there
	// to act on: no such spot, a report already hidden, a problem already resolved.
	ErrNothingToModerate = errors.New("there is nothing to moderate at this address")
	// ErrInvalidStatus is returned for a status a moderator cannot set.
	ErrInvalidStatus = errors.New("a spot is approved or rejected")
)

// DecidedError is returned for a decision on a spot that is no longer
// pending: a spot is decided once (docs/design.md, Lifecycles).
type DecidedError struct{ Status string }

func (e DecidedError) Error() string { return "this spot is already " + e.Status }

// ReportSpotProblem files a problem with a public spot (AC22). What the public
// can see is the public_spots view (migration 0005), nowhere else.
func ReportSpotProblem(ctx context.Context, db *pgxpool.Pool, reporterID, spotID int64, reason string) error {
	tag, err := db.Exec(ctx, `INSERT INTO problem_reports (spot_id, reporter_id, reason)
		SELECT id, $1, $3 FROM public_spots WHERE id = $2`, reporterID, spotID, reason)
	return affected(tag.RowsAffected(), err)
}

// ReportReportProblem files a problem with a public report (AC22).
func ReportReportProblem(ctx context.Context, db *pgxpool.Pool, reporterID, reportID int64, reason string) error {
	tag, err := db.Exec(ctx, `INSERT INTO problem_reports (report_id, reporter_id, reason)
		SELECT id, $1, $3 FROM public_reports WHERE id = $2`, reporterID, reportID, reason)
	return affected(tag.RowsAffected(), err)
}

func affected(n int64, err error) error { return acted(n, err, ErrTargetNotFound) }

// acted returns err, or missing when nothing was affected.
func acted(n int64, err, missing error) error {
	if err != nil {
		return err
	}
	if n == 0 {
		return missing
	}
	return nil
}

// Problem is an open problem report on the queue, with what it is about: a
// spot, or a report (with its spot) that may since have been hidden.
type Problem struct {
	ID           int64
	Reason       string
	CreatedAt    time.Time
	SpotID       int64
	SpotName     string
	ReportID     int64 // 0 for a problem with a spot
	ReportNote   string
	Hidden       bool
	HiddenReason string
}

// OpenProblems lists the open problem reports, oldest first (AC22, AC20).
func OpenProblems(ctx context.Context, db *pgxpool.Pool) ([]Problem, error) {
	rows, err := db.Query(ctx, `SELECT p.id, p.reason, p.created_at, s.id, s.name, coalesce(r.id, 0),
			coalesce(r.note, ''), coalesce(r.hidden, false), coalesce(r.hidden_reason, '')
		FROM problem_reports p
		LEFT JOIN reports r ON r.id = p.report_id
		JOIN spots s ON s.id = coalesce(p.spot_id, r.spot_id)
		WHERE p.status = 'open' ORDER BY p.created_at, p.id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Problem])
}

// moderator returns accounts.ErrPermissionDenied unless actorID is a
// moderator or admin: the domain checks who may act, not only the handler.
func moderator(ctx context.Context, db *pgxpool.Pool, actorID int64) error {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND role IN ('moderator', 'admin'))`, actorID).Scan(&ok)
	if err == nil && !ok {
		return accounts.ErrPermissionDenied
	}
	return err
}

// SetStatus approves or rejects a pending spot (AC19), recording who and
// when in the same statement. A spot is decided once: the update only moves a
// pending spot, so of two decisions at once exactly one lands and the other
// is a DecidedError naming the state (the lifecycle table in docs/design.md).
func SetStatus(ctx context.Context, db *pgxpool.Pool, actorID, spotID int64, status string) error {
	if status != "approved" && status != "rejected" {
		return ErrInvalidStatus
	}
	if err := moderator(ctx, db, actorID); err != nil {
		return err
	}
	tag, err := db.Exec(ctx, `UPDATE spots SET status = $2, moderated_by = $3, moderated_at = now()
		WHERE id = $1 AND status = 'pending'`, spotID, status, actorID)
	if err != nil || tag.RowsAffected() == 1 {
		return err
	}
	var current string
	err = db.QueryRow(ctx, `SELECT status FROM spots WHERE id = $1`, spotID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNothingToModerate
	}
	if err != nil {
		return err
	}
	return DecidedError{Status: current}
}

// Decision is one decided spot in the queue's history.
type Decision struct {
	SpotID      int64
	SpotName    string
	Status      string
	ModeratedAt time.Time
	ModeratedBy string // the moderator's address, or "" once their account is deleted
}

// Decisions lists the latest decisions, newest first.
func Decisions(ctx context.Context, db *pgxpool.Pool) ([]Decision, error) {
	rows, err := db.Query(ctx, `SELECT s.id, s.name, s.status, s.moderated_at, coalesce(u.email, '')
		FROM spots s LEFT JOIN users u ON u.id = s.moderated_by
		WHERE s.moderated_at IS NOT NULL ORDER BY s.moderated_at DESC, s.id DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Decision])
}

// HideReport hides a visible report with a reason (AC20).
func HideReport(ctx context.Context, db *pgxpool.Pool, actorID, reportID int64, reason string) error {
	if err := moderator(ctx, db, actorID); err != nil {
		return err
	}
	tag, err := db.Exec(ctx, `UPDATE reports SET hidden = true, hidden_reason = $2, hidden_at = now() WHERE id = $1 AND NOT hidden`, reportID, reason)
	return acted(tag.RowsAffected(), err, ErrNothingToModerate)
}

// Resolve takes an open problem report off the queue.
func Resolve(ctx context.Context, db *pgxpool.Pool, actorID, problemID int64) error {
	if err := moderator(ctx, db, actorID); err != nil {
		return err
	}
	tag, err := db.Exec(ctx, `UPDATE problem_reports SET status = 'resolved' WHERE id = $1 AND status = 'open'`, problemID)
	return acted(tag.RowsAffected(), err, ErrNothingToModerate)
}

// Package moderation is the queue: problem reports from members (slice 4),
// and approving, rejecting and hiding (slice 5).
package moderation

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrTargetNotFound is returned when the spot or report to flag is not visible.
var ErrTargetNotFound = errors.New("there is nothing to report at this address")

// ReportSpotProblem files a problem with an approved spot (AC22).
func ReportSpotProblem(ctx context.Context, db *pgxpool.Pool, reporterID, spotID int64, reason string) error {
	tag, err := db.Exec(ctx, `INSERT INTO problem_reports (spot_id, reporter_id, reason)
		SELECT id, $1, $3 FROM spots WHERE id = $2 AND status = 'approved'`, reporterID, spotID, reason)
	return inserted(tag.RowsAffected(), err)
}

// ReportReportProblem files a problem with a visible report (AC22).
func ReportReportProblem(ctx context.Context, db *pgxpool.Pool, reporterID, reportID int64, reason string) error {
	tag, err := db.Exec(ctx, `INSERT INTO problem_reports (report_id, reporter_id, reason)
		SELECT r.id, $1, $3 FROM reports r JOIN spots s ON s.id = r.spot_id
		WHERE r.id = $2 AND NOT r.hidden AND s.status = 'approved'`, reporterID, reportID, reason)
	return inserted(tag.RowsAffected(), err)
}

func inserted(n int64, err error) error {
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTargetNotFound
	}
	return nil
}

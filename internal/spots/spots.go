// Package spots is the catalogue: places to work, their attributes and reports.
package spots

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Spot is a place to work.
type Spot struct {
	ID            int64
	Name, Kind    string
	Area, Address string
	OpeningHours  string
	Status        string
}

// ListApproved returns approved spots by name.
func ListApproved(ctx context.Context, db *pgxpool.Pool) ([]Spot, error) {
	rows, err := db.Query(ctx, `SELECT id, name, kind, area, address, opening_hours, status
		FROM spots WHERE status = 'approved' ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Spot])
}

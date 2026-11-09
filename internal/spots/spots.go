// Package spots is the catalogue: places to work, their attributes and reports.
package spots

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSpotNotFound is returned for an id with no visible spot.
var ErrSpotNotFound = errors.New("spot not found")

// Attributes are what matters for working, as the effective values for a spot
// (its visible reports' median or majority, else what its adder declared) or
// as one report's ratings. Scales are in migrations/0002.
type Attributes struct {
	Wifi, Noise                                         int16
	Power, TimeLimit, StepFree, AccessibleToilet, Quiet bool
}

// WifiLabels and NoiseLabels name the scale points for pages and forms.
var (
	WifiLabels  = []string{"None", "Poor", "Usable", "Good", "Excellent"}
	NoiseLabels = []string{"", "Quiet", "Moderate", "Loud"}
)

func (a Attributes) WifiLabel() string  { return label(WifiLabels, int(a.Wifi)) }
func (a Attributes) NoiseLabel() string { return label(NoiseLabels, int(a.Noise)) }

func label(labels []string, i int) string {
	if i < 0 || i >= len(labels) {
		return "Unknown"
	}
	return labels[i]
}

// Spot is a place to work with its effective attributes.
type Spot struct {
	ID            int64
	Name, Kind    string
	Area, Address string
	OpeningHours  string
	Status        string
	ReportCount   int32
	Attributes
}

// Report is one person's dated account of a spot.
type Report struct {
	ID        int64
	Note      string
	CreatedAt time.Time
	Attributes
}

// Filter is a search: an area (town or postcode district) and minimums.
type Filter struct {
	Area                                                  string
	MinWifi, MaxNoise                                     int16 // 0 means any
	Power, NoTimeLimit, StepFree, AccessibleToilet, Quiet bool
}

const spotColumns = `s.id, s.name, s.kind, s.area, s.address, s.opening_hours, s.status, a.report_count,
	a.wifi, a.noise, a.power, a.time_limit, a.step_free, a.accessible_toilet, a.quiet_space`

// Search returns approved spots matching f, by name. An empty area matches
// every area; otherwise every word typed must be one of the area's comma- or
// space-separated words, case-insensitively, so "Leeds, LS6" is found by
// "leeds", "ls6" and "Leeds, LS6".
func Search(ctx context.Context, db *pgxpool.Pool, f Filter) ([]Spot, error) {
	words := strings.Fields(strings.ToLower(strings.NewReplacer(",", " ", ";", " ", "/", " ").Replace(f.Area)))
	rows, err := db.Query(ctx, `SELECT `+spotColumns+`
		FROM spots s JOIN spot_attributes a ON a.spot_id = s.id
		WHERE s.status = 'approved'
		  AND $1::text[] <@ string_to_array(lower(regexp_replace(s.area, '[,;/]', ' ', 'g')), ' ')
		  AND a.wifi >= $2
		  AND ($3 = 0 OR a.noise <= $3)
		  AND (NOT $4 OR a.power)
		  AND (NOT $5 OR NOT a.time_limit)
		  AND (NOT $6 OR a.step_free)
		  AND (NOT $7 OR a.accessible_toilet)
		  AND (NOT $8 OR a.quiet_space)
		ORDER BY s.name, s.id`,
		words, f.MinWifi, f.MaxNoise, f.Power, f.NoTimeLimit, f.StepFree, f.AccessibleToilet, f.Quiet)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Spot])
}

// ListApproved returns every approved spot by name.
func ListApproved(ctx context.Context, db *pgxpool.Pool) ([]Spot, error) {
	return Search(ctx, db, Filter{})
}

// Get returns an approved spot, or ErrSpotNotFound. (Authors and moderators
// seeing their pending spots comes with accounts.)
func Get(ctx context.Context, db *pgxpool.Pool, id int64) (Spot, error) {
	rows, err := db.Query(ctx, `SELECT `+spotColumns+`
		FROM spots s JOIN spot_attributes a ON a.spot_id = s.id
		WHERE s.id = $1 AND s.status = 'approved'`, id)
	if err != nil {
		return Spot{}, err
	}
	spot, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[Spot])
	if errors.Is(err, pgx.ErrNoRows) {
		return Spot{}, ErrSpotNotFound
	}
	return spot, err
}

// VisibleReports returns a spot's reports that moderators haven't hidden, newest first.
func VisibleReports(ctx context.Context, db *pgxpool.Pool, spotID int64) ([]Report, error) {
	rows, err := db.Query(ctx, `SELECT id, note, created_at, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space
		FROM reports WHERE spot_id = $1 AND NOT hidden ORDER BY created_at DESC, id DESC`, spotID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Report])
}

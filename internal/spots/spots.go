// Package spots is the catalogue: places to work, their attributes and reports.
package spots

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	Page                                                  int // 1-based; 0 means the first
}

const spotColumns = `s.id, s.name, s.kind, s.area, s.address, s.opening_hours, s.status, a.report_count,
	a.wifi, a.noise, a.power, a.time_limit, a.step_free, a.accessible_toilet, a.quiet_space`

// Viewer is who is looking: Anonymous for a visitor without an account.
type Viewer struct {
	UserID    int64
	Moderator bool // moderators and admins see every spot (slice 5)
}

// Anonymous is a signed-out visitor.
var Anonymous = Viewer{}

// visible is the one definition of which spots a viewer may see (Search and
// Get; Mine and Pending list by owner and by queue): what the public sees,
// which is the public_spots view and nothing else, a member's own spots in any
// state, and everything for a moderator. Every query that uses it takes the
// viewer's id as $1 and moderator flag as $2.
const visible = `(s.id IN (SELECT id FROM public_spots) OR s.created_by = $1 OR $2)`

// ErrInvalid is returned for a field the database constraints refuse.
var ErrInvalid = errors.New("the spot or report is not valid")

// PageSize is how many spots a search page lists.
const PageSize = 50

// Search returns one page of visible spots matching f, by name, and whether
// a further page exists. An empty area matches every area; otherwise every
// word typed must be one of the area's comma- or space-separated words,
// case-insensitively, so "Leeds, LS6" is found by "leeds", "ls6" and
// "Leeds, LS6".
func Search(ctx context.Context, db *pgxpool.Pool, f Filter) (page []Spot, more bool, err error) {
	words := strings.Fields(strings.ToLower(strings.NewReplacer(",", " ", ";", " ", "/", " ").Replace(f.Area)))
	offset := max(f.Page-1, 0) * PageSize
	// Search lists approved spots only, whoever is looking: a pending spot is
	// reached from its author's account page, not from search (AC14).
	rows, err := db.Query(ctx, `SELECT `+spotColumns+`
		FROM spots s JOIN spot_attributes a ON a.spot_id = s.id
		WHERE `+visible+`
		  AND $3::text[] <@ string_to_array(lower(regexp_replace(s.area, '[,;/]', ' ', 'g')), ' ')
		  AND a.wifi >= $4
		  AND ($5 = 0 OR a.noise <= $5)
		  AND (NOT $6 OR a.power)
		  AND (NOT $7 OR NOT a.time_limit)
		  AND (NOT $8 OR a.step_free)
		  AND (NOT $9 OR a.accessible_toilet)
		  AND (NOT $10 OR a.quiet_space)
		ORDER BY s.name, s.id
		LIMIT $11 OFFSET $12`,
		Anonymous.UserID, Anonymous.Moderator, words, f.MinWifi, f.MaxNoise, f.Power, f.NoTimeLimit, f.StepFree, f.AccessibleToilet, f.Quiet, PageSize+1, offset)
	if err != nil {
		return nil, false, err
	}
	page, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Spot])
	if len(page) > PageSize { // one extra row was asked for, to know whether a next page exists
		return page[:PageSize], true, err
	}
	return page, false, err
}

// ListApproved returns the first page of visible spots by name.
func ListApproved(ctx context.Context, db *pgxpool.Pool) ([]Spot, error) {
	page, _, err := Search(ctx, db, Filter{})
	return page, err
}

// Get returns a spot the viewer may see, or ErrSpotNotFound.
func Get(ctx context.Context, db *pgxpool.Pool, viewer Viewer, id int64) (Spot, error) {
	rows, err := db.Query(ctx, `SELECT `+spotColumns+`
		FROM spots s JOIN spot_attributes a ON a.spot_id = s.id
		WHERE s.id = $3 AND `+visible, viewer.UserID, viewer.Moderator, id)
	if err != nil {
		return Spot{}, err
	}
	spot, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[Spot])
	if errors.Is(err, pgx.ErrNoRows) {
		return Spot{}, ErrSpotNotFound
	}
	return spot, err
}

// VisibleReports returns a spot's reports the public may see (the
// public_reports view: not hidden, on an approved spot), newest first: every
// one, as history; only each member's latest votes (the spot_attributes view).
func VisibleReports(ctx context.Context, db *pgxpool.Pool, spotID int64) ([]Report, error) {
	rows, err := db.Query(ctx, `SELECT id, note, created_at, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space
		FROM public_reports WHERE spot_id = $1 ORDER BY created_at DESC, id DESC`, spotID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Report])
}

// NewSpot is what a member fills in to add a spot. Attributes are declared
// until the first report arrives.
type NewSpot struct {
	Name, Kind, Area, Address, OpeningHours string
	Attributes
}

// Kinds a spot can be, as the database constraint lists them.
var Kinds = []string{"cafe", "library", "coworking", "hotel_lobby", "other"}

// Add stores a pending spot for the author and returns its id (AC14).
func Add(ctx context.Context, db *pgxpool.Pool, authorID int64, n NewSpot) (int64, error) {
	var id int64
	err := db.QueryRow(ctx, `INSERT INTO spots (name, kind, area, address, opening_hours, created_by,
			wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id`,
		n.Name, n.Kind, n.Area, n.Address, n.OpeningHours, authorID,
		n.Wifi, n.Noise, n.Power, n.TimeLimit, n.StepFree, n.AccessibleToilet, n.Quiet).Scan(&id)
	return id, constraint(err)
}

// AddReport stores a member's report on a public spot (AC16). What the public
// can see is the public_spots view (migration 0005), nowhere else.
func AddReport(ctx context.Context, db *pgxpool.Pool, authorID, spotID int64, a Attributes, note string) error {
	tag, err := db.Exec(ctx, `INSERT INTO reports (spot_id, author_id, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space, note)
		SELECT s.id, $1, $3, $4, $5, $6, $7, $8, $9, $10 FROM public_spots s WHERE s.id = $2`,
		authorID, spotID, a.Wifi, a.Noise, a.Power, a.TimeLimit, a.StepFree, a.AccessibleToilet, a.Quiet, note)
	if err != nil {
		return constraint(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSpotNotFound
	}
	return nil
}

// Mine lists the spots a member added, newest first, in any state.
func Mine(ctx context.Context, db *pgxpool.Pool, userID int64) ([]Spot, error) {
	rows, err := db.Query(ctx, `SELECT `+spotColumns+`
		FROM spots s JOIN spot_attributes a ON a.spot_id = s.id
		WHERE s.created_by = $1 ORDER BY s.created_at DESC, s.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Spot])
}

// Pending lists the spots awaiting approval, oldest first, for the queue.
func Pending(ctx context.Context, db *pgxpool.Pool) ([]Spot, error) {
	rows, err := db.Query(ctx, `SELECT `+spotColumns+`
		FROM spots s JOIN spot_attributes a ON a.spot_id = s.id
		WHERE s.status = 'pending' ORDER BY s.created_at, s.id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Spot])
}

// constraint turns a check-constraint violation (SQLSTATE 23514) into
// ErrInvalid; the handler validated first, so this is the backstop.
func constraint(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		return fmt.Errorf("%w: %s", ErrInvalid, pgErr.ConstraintName)
	}
	return err
}

// ReportVisible reports whether the public can see a report (the
// public_reports view), and so may flag it.
func ReportVisible(ctx context.Context, db *pgxpool.Pool, reportID int64) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public_reports WHERE id = $1)`, reportID).Scan(&ok)
	return ok, err
}

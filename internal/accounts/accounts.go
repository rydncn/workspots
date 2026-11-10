// Package accounts is sign-up, sign-in, sessions and passwords. It returns
// named errors and knows nothing about HTTP.
package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidEmail       = errors.New("that does not look like an email address")
	ErrPasswordTooShort   = errors.New("the password must be at least 12 characters")
	ErrPasswordTooLong    = errors.New("the password must be at most 1,024 bytes")
	ErrInvalidCredentials = errors.New("the email address or password is not right")
	ErrSessionNotFound    = errors.New("session not found")
	ErrPermissionDenied   = errors.New("you do not have permission to do this")
	ErrInvalidRole        = errors.New("that is not a role")
	ErrUserNotFound       = errors.New("no account has that address")
)

// Roles a user can have, as the database constraint lists them.
var Roles = []string{"member", "moderator", "admin"}

// SessionLifetime is how long a session lasts from sign-in (docs/design.md, retention).
const SessionLifetime = 30 * 24 * time.Hour

// expired is the one definition of a session that has run out (rule 19): a
// live session is NOT expired. Expired rows are deleted whenever sessions change.
const expired = `expires_at <= now()`

// User is a signed-up person.
type User struct {
	ID    int64
	Email string
	Role  string
}

// ValidatePassword applies the length rules, by character for the minimum and
// by byte for the maximum, before anything is hashed.
func ValidatePassword(password string) error {
	if len(password) > 1024 {
		return ErrPasswordTooLong
	}
	if utf8.RuneCountInString(password) < 12 {
		return ErrPasswordTooShort
	}
	return nil
}

// ValidateEmail is deliberately loose: something@something, no spaces, short
// enough, and nothing Postgres would refuse (a NUL byte, invalid UTF-8).
func ValidateEmail(email string) error {
	at := strings.Index(email, "@")
	if len(email) < 3 || len(email) > 254 || at < 1 || at == len(email)-1 || strings.ContainsAny(email, " \t\r\n\x00") || !utf8.ValidString(email) {
		return ErrInvalidEmail
	}
	return nil
}

// SignUp creates a member, or does nothing when the address is taken. The
// second result says which, so the caller can send the right mail while
// showing the same page either way.
func SignUp(ctx context.Context, db *pgxpool.Pool, email, password string) (created bool, err error) {
	if err := ValidateEmail(email); err != nil {
		return false, err
	}
	if err := ValidatePassword(password); err != nil {
		return false, err
	}
	var hash string
	var hashErr error
	if err := withHashSlot(ctx, func() { hash, hashErr = hashPassword(password) }); err != nil {
		return false, err
	}
	if hashErr != nil {
		return false, hashErr
	}
	tag, err := db.Exec(ctx, `INSERT INTO users (email, password_hash) SELECT $1, $2
		WHERE NOT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($1))`, email, hash)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // two sign-ups for one address at once: the unique index decides
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Authenticate returns the user for a correct email and password, or
// ErrInvalidCredentials for either being wrong, in about the same time.
func Authenticate(ctx context.Context, db *pgxpool.Pool, email, password string) (User, error) {
	if len(password) > 1024 { // never hash an oversized password
		return User{}, ErrInvalidCredentials
	}
	var u User
	var hash string
	err := db.QueryRow(ctx, `SELECT id, email, role, password_hash FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&u.ID, &u.Email, &u.Role, &hash)
	unknown := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !unknown {
		return User{}, err
	}
	if unknown {
		hash = dummyHash // same work for an unknown address
	}
	var ok bool
	if err := withHashSlot(ctx, func() { ok = verifyPassword(hash, password) }); err != nil {
		return User{}, err
	}
	if unknown || !ok {
		return User{}, ErrInvalidCredentials
	}
	return u, nil
}

// CreateSession starts a session and returns the token for the cookie. Only
// its hash is stored.
func CreateSession(ctx context.Context, db *pgxpool.Pool, userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	// Retention (docs/design.md): expired sessions are deleted, not just ignored.
	if _, err := db.Exec(ctx, `DELETE FROM sessions WHERE `+expired); err != nil {
		return "", err
	}
	_, err := db.Exec(ctx, `INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, now() + $3::interval)`,
		SessionID(token), userID, SessionLifetime.String())
	return token, err
}

// SessionID is the stored form of a token.
func SessionID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SessionUser returns the user a live session token belongs to.
func SessionUser(ctx context.Context, db *pgxpool.Pool, token string) (User, error) {
	var u User
	err := db.QueryRow(ctx, `SELECT u.id, u.email, u.role FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND NOT (s.`+expired+`)`, SessionID(token)).Scan(&u.ID, &u.Email, &u.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrSessionNotFound
	}
	return u, err
}

// EndSession deletes one session, and any expired ones while it is at it; a
// token that is already gone is not an error.
func EndSession(ctx context.Context, db *pgxpool.Pool, token string) error {
	_, err := db.Exec(ctx, `DELETE FROM sessions WHERE id = $1 OR `+expired, SessionID(token))
	return err
}

// EndOtherSessions deletes every session of the user except the one for keepToken.
func EndOtherSessions(ctx context.Context, db *pgxpool.Pool, userID int64, keepToken string) error {
	_, err := db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND id <> $2`, userID, SessionID(keepToken))
	return err
}

// confirm checks the account's own password before a sensitive change, or
// returns ErrInvalidCredentials.
func confirm(ctx context.Context, db *pgxpool.Pool, userID int64, password string) error {
	var hash string
	if err := db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return err
	}
	if len(password) > 1024 {
		return ErrInvalidCredentials
	}
	var ok bool
	if err := withHashSlot(ctx, func() { ok = verifyPassword(hash, password) }); err != nil {
		return err
	}
	if !ok {
		return ErrInvalidCredentials
	}
	return nil
}

// ChangePassword replaces the password once the current one is confirmed. The
// new password's length is checked first, before any hashing (AC7).
func ChangePassword(ctx context.Context, db *pgxpool.Pool, userID int64, current, next string) error {
	if err := ValidatePassword(next); err != nil {
		return err
	}
	if err := confirm(ctx, db, userID, current); err != nil {
		return err
	}
	var newHash string
	var hashErr error
	if err := withHashSlot(ctx, func() { newHash, hashErr = hashPassword(next) }); err != nil {
		return err
	}
	if hashErr != nil {
		return hashErr
	}
	_, err := db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, newHash)
	return err
}

// Delete removes the account once its password is confirmed (AC11, ADR 0011).
// With the user row go its sessions, reports and problem reports (ON DELETE
// CASCADE) and any spot of theirs not yet approved; an approved spot describes
// a place, so it stays with no author, and a decision they made as a moderator
// keeps no name (ON DELETE SET NULL). One statement, so all or nothing.
//
// The withdrawal guard reads the row's own status rather than the public_spots
// view on purpose: a subquery is evaluated on the statement's snapshot, so a
// spot approved while this statement waited on its row would still be deleted;
// a predicate on the row itself is re-checked on the row's new version
// (scripts/lifecycle-reads.txt lists it).
func Delete(ctx context.Context, db *pgxpool.Pool, userID int64, password string) error {
	if err := confirm(ctx, db, userID, password); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `WITH withdrawn AS (
			DELETE FROM spots WHERE created_by = $1 AND status <> 'approved')
		DELETE FROM users WHERE id = $1`, userID)
	return err
}

// SetRole lets an admin give the account at email a role (AC21).
func SetRole(ctx context.Context, db *pgxpool.Pool, actorID int64, email, role string) (int64, error) {
	var admin bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND role = 'admin')`, actorID).Scan(&admin); err != nil {
		return 0, err
	}
	if !admin {
		return 0, ErrPermissionDenied
	}
	return GrantRole(ctx, db, email, role)
}

// GrantRole gives the account at email a role with no check on who asks (the
// operator's grant-role command, which makes the first admin) and returns the
// account's id. A change of privilege ends the account's sessions
// (.claude/rules/api.md): the person signs in again with the new role.
func GrantRole(ctx context.Context, db *pgxpool.Pool, email, role string) (int64, error) {
	if !slices.Contains(Roles, role) {
		return 0, ErrInvalidRole
	}
	var id int64
	err := db.QueryRow(ctx, `WITH changed AS (UPDATE users SET role = $2 WHERE lower(email) = lower($1) RETURNING id),
		ended AS (DELETE FROM sessions WHERE user_id IN (SELECT id FROM changed))
		SELECT id FROM changed`, email, role).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrUserNotFound
	}
	return id, err
}

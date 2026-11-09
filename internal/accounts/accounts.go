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
)

// SessionLifetime is how long a session lasts from sign-in (docs/design.md, retention).
const SessionLifetime = 30 * 24 * time.Hour

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

// ValidateEmail is deliberately loose: something@something, no spaces, short enough.
func ValidateEmail(email string) error {
	at := strings.Index(email, "@")
	if len(email) < 3 || len(email) > 254 || at < 1 || at == len(email)-1 || strings.ContainsAny(email, " \t\r\n") {
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
	hash, err := hashPassword(password)
	if err != nil {
		return false, err
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
	if errors.Is(err, pgx.ErrNoRows) {
		verifyPassword(dummyHash, password)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	if !verifyPassword(hash, password) {
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
	if _, err := db.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`); err != nil {
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
		WHERE s.id = $1 AND s.expires_at > now()`, SessionID(token)).Scan(&u.ID, &u.Email, &u.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrSessionNotFound
	}
	return u, err
}

// EndSession deletes one session, and any expired ones while it is at it; a
// token that is already gone is not an error.
func EndSession(ctx context.Context, db *pgxpool.Pool, token string) error {
	_, err := db.Exec(ctx, `DELETE FROM sessions WHERE id = $1 OR expires_at <= now()`, SessionID(token))
	return err
}

// EndOtherSessions deletes every session of the user except the one for keepToken.
func EndOtherSessions(ctx context.Context, db *pgxpool.Pool, userID int64, keepToken string) error {
	_, err := db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND id <> $2`, userID, SessionID(keepToken))
	return err
}

// ChangePassword replaces the password once the current one is confirmed.
func ChangePassword(ctx context.Context, db *pgxpool.Pool, userID int64, current, next string) error {
	var hash string
	if err := db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return err
	}
	if len(current) > 1024 || !verifyPassword(hash, current) {
		return ErrInvalidCredentials
	}
	if err := ValidatePassword(next); err != nil {
		return err
	}
	newHash, err := hashPassword(next)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, newHash)
	return err
}

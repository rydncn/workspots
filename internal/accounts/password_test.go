package accounts

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	t.Parallel()
	h, err := hashPassword("correct horse battery")
	if err != nil || !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash %q, %v", h, err)
	}
	if !verifyPassword(h, "correct horse battery") {
		t.Error("right password refused")
	}
	if verifyPassword(h, "correct horse batterx") {
		t.Error("wrong password accepted")
	}
	if verifyPassword("$argon2id$v=19$m=19456,t=2,p=1$bad$bad", "x") || verifyPassword("", "x") {
		t.Error("malformed hash accepted")
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()
	if ValidatePassword("elevenchars") == nil || ValidatePassword(strings.Repeat("é", 12)) != nil {
		t.Error("minimum is 12 characters, not bytes")
	}
	if ValidatePassword(strings.Repeat("x", 1025)) != ErrPasswordTooLong || ValidatePassword(strings.Repeat("x", 1024)) != nil {
		t.Error("maximum is 1,024 bytes")
	}
}

// ValidateEmail refuses what Postgres would refuse later (a NUL byte, invalid
// UTF-8), so the form answers 422 rather than a 500 (security audit, slice 6).
func TestValidateEmailRefusesWhatPostgresWould(t *testing.T) {
	t.Parallel()
	for _, email := range []string{"nul\x00@example.test", "bad\xff@example.test", "a b@example.test", "@example.test", "nobody"} {
		if ValidateEmail(email) == nil {
			t.Errorf("%q accepted", email)
		}
	}
	if err := ValidateEmail("someone@example.test"); err != nil {
		t.Error(err)
	}
}

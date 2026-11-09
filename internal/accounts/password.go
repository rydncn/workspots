package accounts

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (ADR 0012): OWASP's current recommendation. Old hashes
// carry their own parameters in the PHC string, so these can be raised later.
const (
	argonTime    = 2
	argonMemory  = 19 * 1024 // KiB
	argonThreads = 1
	argonKeyLen  = 32
	saltLen      = 16
)

// hashGate caps how many argon2id computations run at once (each takes 19 MiB),
// so a burst of sign-ins cannot exhaust memory. A request waits for a slot
// until its context ends (ADR 0012).
var hashGate = make(chan struct{}, runtime.NumCPU())

func withHashSlot(ctx context.Context, f func()) error {
	select {
	case hashGate <- struct{}{}:
		defer func() { <-hashGate }()
		f()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// hashPassword returns a PHC-encoded argon2id hash.
func hashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// verifyPassword reports whether password matches the PHC-encoded hash.
func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	b64 := base64.RawStdEncoding
	salt, err1 := b64.DecodeString(parts[4])
	want, err2 := b64.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is verified against when the address is unknown, so a refusal
// takes the same time whether or not the account exists.
var dummyHash = func() string {
	h, err := hashPassword("dummy password for timing")
	if err != nil {
		panic(err)
	}
	return h
}()

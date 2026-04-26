package security

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("Admin@123456")
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	if !strings.HasPrefix(hash, "pbkdf2-sha256$") {
		t.Fatalf("expected pbkdf2 hash, got %q", hash)
	}
	if !VerifyPassword(hash, "Admin@123456") {
		t.Fatalf("expected password verification to succeed")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatalf("expected password verification to fail")
	}
}

func TestVerifyLegacySHA256PasswordHash(t *testing.T) {
	salt := []byte("0123456789abcdef")
	hash := fmt.Sprintf("sha256$%s$%s",
		hex.EncodeToString(salt),
		hex.EncodeToString(deriveLegacyPasswordHash("Admin@123456", salt)),
	)
	if !VerifyPassword(hash, "Admin@123456") {
		t.Fatalf("expected legacy password verification to succeed")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatalf("expected legacy password verification to fail")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	cases := []string{
		"",
		"sha256$bad-salt$bad-hash",
		"pbkdf2-sha256$0$30313233343536373839616263646566$00",
		"pbkdf2-sha256$1000001$30313233343536373839616263646566$00",
		"pbkdf2-sha256$not-number$30313233343536373839616263646566$00",
		"unknown$30313233343536373839616263646566$00",
	}
	for _, tc := range cases {
		if VerifyPassword(tc, "Admin@123456") {
			t.Fatalf("expected malformed hash %q to fail", tc)
		}
	}
}

func TestTokenRoundTrip(t *testing.T) {
	access, refresh, err := IssueTokenPair("secret", 1, "admin", []string{"admin"}, time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("issue token pair failed: %v", err)
	}
	accessClaims, err := ParseToken("secret", access)
	if err != nil {
		t.Fatalf("parse access token failed: %v", err)
	}
	if accessClaims.Kind != "access" || accessClaims.UserID != 1 {
		t.Fatalf("unexpected access claims: %+v", accessClaims)
	}
	refreshClaims, err := ParseToken("secret", refresh)
	if err != nil {
		t.Fatalf("parse refresh token failed: %v", err)
	}
	if refreshClaims.Kind != "refresh" || refreshClaims.Username != "admin" {
		t.Fatalf("unexpected refresh claims: %+v", refreshClaims)
	}
}

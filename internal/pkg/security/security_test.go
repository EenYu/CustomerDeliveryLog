package security

import (
	"testing"
	"time"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("Admin@123456")
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	if !VerifyPassword(hash, "Admin@123456") {
		t.Fatalf("expected password verification to succeed")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatalf("expected password verification to fail")
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

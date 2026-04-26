package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type TokenClaims struct {
	UserID   int64    `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	Kind     string   `json:"kind"`
	Exp      int64    `json:"exp"`
}

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

func SignToken(secret string, claims TokenClaims) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := encodedHeader + "." + encodedPayload
	signature := sign(secret, unsigned)
	return unsigned + "." + signature, nil
}

func ParseToken(secret, token string) (TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return TokenClaims{}, ErrInvalidToken
	}
	unsigned := parts[0] + "." + parts[1]
	expected := sign(secret, unsigned)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return TokenClaims{}, ErrInvalidToken
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	var claims TokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	if claims.Exp <= time.Now().Unix() {
		return TokenClaims{}, ErrExpiredToken
	}
	return claims, nil
}

func IssueTokenPair(secret string, userID int64, username string, roles []string, accessTTL, refreshTTL time.Duration) (string, string, error) {
	now := time.Now()
	access, err := SignToken(secret, TokenClaims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		Kind:     "access",
		Exp:      now.Add(accessTTL).Unix(),
	})
	if err != nil {
		return "", "", err
	}
	refresh, err := SignToken(secret, TokenClaims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		Kind:     "refresh",
		Exp:      now.Add(refreshTTL).Unix(),
	})
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func sign(secret, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func BearerToken(authorization string) (string, error) {
	if authorization == "" {
		return "", ErrInvalidToken
	}
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("%w: bad authorization header", ErrInvalidToken)
	}
	return strings.TrimSpace(parts[1]), nil
}

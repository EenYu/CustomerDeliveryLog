package security

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	legacyPasswordIterations = 60000
	passwordAlgorithm        = "pbkdf2-sha256"
	passwordIterations       = 210000
	passwordMaxIterations    = 1000000
	passwordSaltBytes        = 16
	passwordHashBytes        = 32
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash, err := derivePasswordHash(password, salt, passwordIterations)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s$%d$%s$%s", passwordAlgorithm, passwordIterations, hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

func VerifyPassword(encodedHash, password string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) == 3 && parts[0] == "sha256" {
		return verifyLegacyPasswordHash(parts, password)
	}
	if len(parts) != 4 || parts[0] != passwordAlgorithm {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 || iterations > passwordMaxIterations {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual, err := derivePasswordHash(password, salt, iterations)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(expected, actual) == 1
}

func derivePasswordHash(password string, salt []byte, iterations int) ([]byte, error) {
	return pbkdf2.Key(sha256.New, password, salt, iterations, passwordHashBytes)
}

func verifyLegacyPasswordHash(parts []string, password string) bool {
	salt, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	actual := deriveLegacyPasswordHash(password, salt)
	return subtle.ConstantTimeCompare(expected, actual) == 1
}

func deriveLegacyPasswordHash(password string, salt []byte) []byte {
	buf := append([]byte{}, salt...)
	buf = append(buf, []byte(password)...)
	hash := sha256.Sum256(buf)
	out := hash[:]
	for i := 0; i < legacyPasswordIterations; i++ {
		data := append([]byte{}, out...)
		data = append(data, salt...)
		sum := sha256.Sum256(data)
		out = sum[:]
	}
	return append([]byte{}, out...)
}

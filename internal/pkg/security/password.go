package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
)

const passwordIterations = 60000

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := derivePasswordHash(password, salt)
	return fmt.Sprintf("sha256$%s$%s", hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

func VerifyPassword(encodedHash, password string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 3 {
		return false
	}
	algo, saltHex, hashHex := parts[0], parts[1], parts[2]
	if algo != "sha256" {
		return false
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}
	actual := derivePasswordHash(password, salt)
	return subtle.ConstantTimeCompare(expected, actual) == 1
}

func derivePasswordHash(password string, salt []byte) []byte {
	buf := append([]byte{}, salt...)
	buf = append(buf, []byte(password)...)
	hash := sha256.Sum256(buf)
	out := hash[:]
	for i := 0; i < passwordIterations; i++ {
		data := append([]byte{}, out...)
		data = append(data, salt...)
		sum := sha256.Sum256(data)
		out = sum[:]
	}
	return append([]byte{}, out...)
}

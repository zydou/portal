package password

import (
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const passLen = 16
const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Generate generates a random password prefixed with the supplied id.
// The password consists of the relay-allocated id followed by 16
// cryptographically random alphanumeric characters, yielding ~95 bits
// of entropy — making brute-force attacks infeasible.
func Generate(id int) (string, error) {
	b := make([]byte, passLen)
	if _, err := crypto_rand.Read(b); err != nil {
		return "", fmt.Errorf("creating rng: %w", err)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return fmt.Sprintf("%d-%s", id, string(b)), nil
}

func Hashed(password string) string {
	h := sha256.New()
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

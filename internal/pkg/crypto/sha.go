package crypto

import (
	"crypto/sha512"
	"encoding/hex"
)

// GenerateSHA generates SHA from string
// Always returns a hash value, even for empty strings, to ensure consistent behavior
// and avoid issues with string matching operations (e.g., strings.Contains(str, "") always returns true)
func GenerateSHA(data string) string {
	hash := sha512.Sum512_256([]byte(data))
	return hex.EncodeToString(hash[:])
}

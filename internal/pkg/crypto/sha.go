package crypto

import (
	"crypto/sha512"
	"encoding/hex"
)

// GenerateSHA generates SHA from string
func GenerateSHA(data string) string {
	hash := sha512.Sum512_256([]byte(data))
	return hex.EncodeToString(hash[:])
}

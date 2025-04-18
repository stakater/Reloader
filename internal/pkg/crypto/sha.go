package crypto

import (
	"crypto/sha512"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

// GenerateSHA generates SHA from string
func GenerateSHA(data string) string {
	hasher := sha512.New()
	_, err := io.WriteString(hasher, data)
	if err != nil {
		logrus.Errorf("Unable to write data in hash writer %v", err)
	}
	sha := hasher.Sum(nil)
	return fmt.Sprintf("%x", sha)
}

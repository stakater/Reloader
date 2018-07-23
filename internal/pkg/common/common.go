package common

import (
	"bytes"
	"math/rand"
	"strings"
	"time"
)

var (
	letters = []rune("abcdefghijklmnopqrstuvwxyz")
)

const (
	// ConfigmapUpdateOnChangeAnnotation is an annotation to detect changes in configmaps
	ConfigmapUpdateOnChangeAnnotation = "configmap.reloader.stakater.com/reload"
	// SecretUpdateOnChangeAnnotation is an annotation to detect changes in secrets
	SecretUpdateOnChangeAnnotation = "secret.reloader.stakater.com/reload"
	// ConfigmapEnvarPostfix is a postfix for configmap envVar
	ConfigmapEnvarPostfix = "_CONFIGMAP"
	// SecretEnvarPostfix is a postfix for secret envVar
	SecretEnvarPostfix = "_SECRET"
	// EnvVarPrefix is a Prefix for environment variable
	EnvVarPrefix = "STAKATER_"
)

// ConvertToEnvVarName converts the given text into a usable env var
// removing any special chars with '_' and transforming text to upper case
func ConvertToEnvVarName(text string) string {
	var buffer bytes.Buffer
	upper := strings.ToUpper(text)
	lastCharValid := false
	for i := 0; i < len(upper); i++ {
		ch := upper[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			buffer.WriteString(string(ch))
			lastCharValid = true
		} else {
			if lastCharValid {
				buffer.WriteString("_")
			}
			lastCharValid = false
		}
	}
	return buffer.String()
}

// RandSeq generates a random sequence
func RandSeq(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

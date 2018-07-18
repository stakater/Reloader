package handler

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
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

// ConvertConfigmapToSHA generates SHA for configmap data
func ConvertConfigmapToSHA(cm *v1.ConfigMap) string {
	logrus.Infof("Generating SHA for configmap data")
	values := []string{}
	for k, v := range cm.Data {
		values = append(values, k+"="+v)
	}
	sort.Strings(values)
	sha := GenerateSHA(strings.Join(values, ";"))
	logrus.Infof("SHA for configmap data: %s", sha)
	return sha
}

// ConvertSecretToSHA generates SHA for secret data
func ConvertSecretToSHA(se *v1.Secret) string {
	logrus.Infof("Generating SHA for secret data")
	values := []string{}
	for k, v := range se.Data {
		values = append(values, k+"="+string(v[:]))
	}
	sort.Strings(values)
	sha := GenerateSHA(strings.Join(values, ";"))
	logrus.Infof("SHA for secret data: %s", sha)
	return sha
}

// GenerateSHA generates SHA from string
func GenerateSHA(data string) string {
	hasher := sha1.New()
	io.WriteString(hasher, data)
	sha := hasher.Sum(nil)
	return fmt.Sprintf("%x", sha)
}

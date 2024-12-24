package util

import (
	"bytes"
	"encoding/base64"
	"sort"
	"strings"

	"github.com/stakater/Reloader/internal/pkg/crypto"
	v1 "k8s.io/api/core/v1"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
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

func GetSHAfromConfigmap(configmap *v1.ConfigMap) string {
	values := []string{}
	for k, v := range configmap.Data {
		values = append(values, k+"="+v)
	}
	for k, v := range configmap.BinaryData {
		values = append(values, k+"="+base64.StdEncoding.EncodeToString(v))
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}

func GetSHAfromSecret(data map[string][]byte) string {
	values := []string{}
	for k, v := range data {
		values = append(values, k+"="+string(v[:]))
	}
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}

func GetSHAfromSecretProviderClassPodStatus(data csiv1.SecretProviderClassPodStatusStatus) string {
	values := []string{}
	for _, v := range data.Objects {
		values = append(values, v.ID+"="+v.Version)
	}
	values = append(values, "SecretProviderClassName="+data.SecretProviderClassName)
	sort.Strings(values)
	return crypto.GenerateSHA(strings.Join(values, ";"))
}

type List []string

type Map map[string]string

func (l *List) Contains(s string) bool {
	for _, v := range *l {
		if v == s {
			return true
		}
	}
	return false
}

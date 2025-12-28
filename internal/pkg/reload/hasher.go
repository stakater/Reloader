// Package reload provides core reload logic for ConfigMaps and Secrets.
package reload

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Hasher computes content hashes for ConfigMaps and Secrets.
type Hasher struct{}

// NewHasher creates a new Hasher instance.
func NewHasher() *Hasher {
	return &Hasher{}
}

// HashConfigMap computes a SHA1 hash of the ConfigMap's data and binaryData.
func (h *Hasher) HashConfigMap(cm *corev1.ConfigMap) string {
	if cm == nil {
		return h.computeSHA("")
	}
	return h.hashConfigMapData(cm.Data, cm.BinaryData)
}

// HashSecret computes a SHA1 hash of the Secret's data.
func (h *Hasher) HashSecret(secret *corev1.Secret) string {
	if secret == nil {
		return h.computeSHA("")
	}
	return h.hashSecretData(secret.Data)
}

func (h *Hasher) hashConfigMapData(data map[string]string, binaryData map[string][]byte) string {
	values := make([]string, 0, len(data)+len(binaryData))

	for k, v := range data {
		values = append(values, k+"="+v)
	}

	for k, v := range binaryData {
		values = append(values, k+"="+base64.StdEncoding.EncodeToString(v))
	}

	sort.Strings(values)
	return h.computeSHA(strings.Join(values, ";"))
}

func (h *Hasher) hashSecretData(data map[string][]byte) string {
	values := make([]string, 0, len(data))

	for k, v := range data {
		values = append(values, k+"="+string(v))
	}

	sort.Strings(values)
	return h.computeSHA(strings.Join(values, ";"))
}

func (h *Hasher) computeSHA(data string) string {
	hasher := sha1.New()
	_, _ = io.WriteString(hasher, data)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// EmptyHash returns an empty string to signal resource deletion.
func (h *Hasher) EmptyHash() string {
	return ""
}

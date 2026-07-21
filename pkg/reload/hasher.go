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
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
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

// HashSecretProviderClass computes a SHA1 hash of a SecretProviderClassPodStatus
// status: the sorted set of object ID=Version entries plus the SPC name.
// This mirrors master's util.GetSHAfromSecretProviderClassPodStatus exactly.
func (h *Hasher) HashSecretProviderClass(status csiv1.SecretProviderClassPodStatusStatus) string {
	values := make([]string, 0, len(status.Objects)+1)
	for _, obj := range status.Objects {
		values = append(values, obj.ID+"="+obj.Version)
	}
	values = append(values, "SecretProviderClassName="+status.SecretProviderClassName)
	sort.Strings(values)
	return h.computeSHA(strings.Join(values, ";"))
}

// EmptyHash returns an empty string to signal resource deletion.
func (h *Hasher) EmptyHash() string {
	return ""
}

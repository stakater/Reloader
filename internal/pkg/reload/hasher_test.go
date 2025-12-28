package reload

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestHasher_HashConfigMap(t *testing.T) {
	hasher := NewHasher()

	tests := []struct {
		name     string
		cm       *corev1.ConfigMap
		wantHash string
	}{
		{
			name: "empty configmap",
			cm: &corev1.ConfigMap{
				Data:       nil,
				BinaryData: nil,
			},
			wantHash: hasher.EmptyHash(),
		},
		{
			name: "configmap with data",
			cm: &corev1.ConfigMap{
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			// Hash should be deterministic
			wantHash: hasher.HashConfigMap(&corev1.ConfigMap{
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			}),
		},
		{
			name: "configmap with binary data",
			cm: &corev1.ConfigMap{
				BinaryData: map[string][]byte{
					"binary1": []byte("binaryvalue1"),
				},
			},
			wantHash: hasher.HashConfigMap(&corev1.ConfigMap{
				BinaryData: map[string][]byte{
					"binary1": []byte("binaryvalue1"),
				},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasher.HashConfigMap(tt.cm)
			if got != tt.wantHash {
				t.Errorf("HashConfigMap() = %v, want %v", got, tt.wantHash)
			}
		})
	}
}

func TestHasher_HashConfigMap_Deterministic(t *testing.T) {
	hasher := NewHasher()

	cm := &corev1.ConfigMap{
		Data: map[string]string{
			"z-key": "value-z",
			"a-key": "value-a",
			"m-key": "value-m",
		},
	}

	// Hash should be the same regardless of iteration order
	hash1 := hasher.HashConfigMap(cm)
	hash2 := hasher.HashConfigMap(cm)
	hash3 := hasher.HashConfigMap(cm)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash is not deterministic: %s, %s, %s", hash1, hash2, hash3)
	}
}

func TestHasher_HashConfigMap_DifferentValues(t *testing.T) {
	hasher := NewHasher()

	cm1 := &corev1.ConfigMap{
		Data: map[string]string{
			"key": "value1",
		},
	}

	cm2 := &corev1.ConfigMap{
		Data: map[string]string{
			"key": "value2",
		},
	}

	hash1 := hasher.HashConfigMap(cm1)
	hash2 := hasher.HashConfigMap(cm2)

	if hash1 == hash2 {
		t.Errorf("Different values should produce different hashes")
	}
}

func TestHasher_HashSecret(t *testing.T) {
	hasher := NewHasher()

	tests := []struct {
		name     string
		secret   *corev1.Secret
		wantHash string
	}{
		{
			name: "empty secret",
			secret: &corev1.Secret{
				Data: nil,
			},
			wantHash: hasher.EmptyHash(),
		},
		{
			name: "secret with data",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
			},
			wantHash: hasher.HashSecret(&corev1.Secret{
				Data: map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasher.HashSecret(tt.secret)
			if got != tt.wantHash {
				t.Errorf("HashSecret() = %v, want %v", got, tt.wantHash)
			}
		})
	}
}

func TestHasher_HashSecret_Deterministic(t *testing.T) {
	hasher := NewHasher()

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"z-key": []byte("value-z"),
			"a-key": []byte("value-a"),
			"m-key": []byte("value-m"),
		},
	}

	// Hash should be the same regardless of iteration order
	hash1 := hasher.HashSecret(secret)
	hash2 := hasher.HashSecret(secret)
	hash3 := hasher.HashSecret(secret)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash is not deterministic: %s, %s, %s", hash1, hash2, hash3)
	}
}

func TestHasher_HashSecret_DifferentValues(t *testing.T) {
	hasher := NewHasher()

	secret1 := &corev1.Secret{
		Data: map[string][]byte{
			"key": []byte("value1"),
		},
	}

	secret2 := &corev1.Secret{
		Data: map[string][]byte{
			"key": []byte("value2"),
		},
	}

	hash1 := hasher.HashSecret(secret1)
	hash2 := hasher.HashSecret(secret2)

	if hash1 == hash2 {
		t.Errorf("Different values should produce different hashes")
	}
}

func TestHasher_EmptyHash(t *testing.T) {
	hasher := NewHasher()

	emptyHash := hasher.EmptyHash()
	if emptyHash == "" {
		t.Error("EmptyHash should not be empty string")
	}

	// Empty ConfigMap should match EmptyHash
	cm := &corev1.ConfigMap{}
	if hasher.HashConfigMap(cm) != emptyHash {
		t.Error("Empty ConfigMap hash should equal EmptyHash")
	}

	// Empty Secret should match EmptyHash
	secret := &corev1.Secret{}
	if hasher.HashSecret(secret) != emptyHash {
		t.Error("Empty Secret hash should equal EmptyHash")
	}
}

func TestHasher_NilInput(t *testing.T) {
	hasher := NewHasher()

	// Test nil ConfigMap
	cmHash := hasher.HashConfigMap(nil)
	if cmHash != hasher.EmptyHash() {
		t.Errorf("nil ConfigMap should return EmptyHash, got %s", cmHash)
	}

	// Test nil Secret
	secretHash := hasher.HashSecret(nil)
	if secretHash != hasher.EmptyHash() {
		t.Errorf("nil Secret should return EmptyHash, got %s", secretHash)
	}
}

package util

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestConvertToEnvVarName(t *testing.T) {
	data := "www.stakater.com"
	envVar := ConvertToEnvVarName(data)
	if envVar != "WWW_STAKATER_COM" {
		t.Errorf("Failed to convert data into environment variable")
	}
}

func TestGetHashFromConfigMap(t *testing.T) {
	data := map[*v1.ConfigMap]string{
		{
			Data: map[string]string{"test": "test"},
		}: "Only Data",
		{
			Data:       map[string]string{"test": "test"},
			BinaryData: map[string][]byte{"bintest": []byte("test")},
		}: "Both Data and BinaryData",
		{
			BinaryData: map[string][]byte{"bintest": []byte("test")},
		}: "Only BinaryData",
	}
	converted := map[string]string{}
	for cm, cmName := range data {
		converted[cmName] = GetSHAfromConfigmap(cm)
	}

	// Test that the has for each configmap is really unique
	for cmName, cmHash := range converted {
		count := 0
		for _, cmHash2 := range converted {
			if cmHash == cmHash2 {
				count++
			}
		}
		if count > 1 {
			t.Errorf("Found duplicate hashes for %v", cmName)
		}
	}
}

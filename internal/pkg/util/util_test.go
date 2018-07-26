package util

import (
	"testing"
)

func TestConvertToEnvVarName(t *testing.T) {
	data := "www.stakater.com"
	envVar := ConvertToEnvVarName(data)
	if envVar != "WWW_STAKATER_COM" {
		t.Errorf("Failed to convert data into environment variable")
	}
}

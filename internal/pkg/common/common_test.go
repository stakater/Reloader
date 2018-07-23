package common

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

func TestRandSeq(t *testing.T) {
	data := RandSeq(5)
	newData := RandSeq(5)
	if data == newData {
		t.Errorf("Random sequence generator does not work correctly")
	}
}

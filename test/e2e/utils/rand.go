package utils

import (
	"math/rand"
	"time"
)

const letters = "abcdefghijklmnopqrstuvwxyz"

var randSource = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec

// RandSeq generates a random lowercase string of length n.
// This is useful for creating unique resource names in tests.
func RandSeq(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[randSource.Intn(len(letters))]
	}
	return string(b)
}

// RandName generates a unique name with the given prefix.
// Format: prefix-xxxxx where x is a random lowercase letter.
func RandName(prefix string) string {
	return prefix + "-" + RandSeq(5)
}

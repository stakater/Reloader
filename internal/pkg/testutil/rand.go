package testutil

import (
	"math/rand/v2"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

// RandSeq generates a random string of the specified length.
func RandSeq(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.IntN(len(letterBytes))]
	}
	return string(b)
}

package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
)

type strings string

const Strings strings = ""

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func (strings) ToFloat64(s string) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	return 0
}

func (strings) Nullable(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// Random generates a cryptographically secure random string of length l
// using characters from charset. Panics if unable to read from crypto/rand,
// which should only occur if the system's random source is unavailable.
func (strings) Random(l int) string {
	b := make([]byte, l)
	for i := range b {
		// Use crypto/rand for cryptographically secure random number generation
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			panic(fmt.Sprintf("failed to generate random string: %v", err))
		}
		b[i] = charset[num.Int64()]
	}

	return string(b)
}

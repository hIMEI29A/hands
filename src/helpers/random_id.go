package helpers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// defaultIdLength is the number of hex characters of a generated ID.
const defaultIdLength = 24

// generateRandomID returns a random lowercase hex string of exactly length
// characters. It reads (length+1)/2 bytes from crypto/rand, hex-encodes them
// and truncates the result to length, so odd lengths work as well.
//
// It returns an empty string and no error when length is not positive. A read
// error of the random source is returned to the caller.
func generateRandomID(length int) (string, error) {
	if length <= 0 {
		return "", nil
	}

	buf := make([]byte, (length+1)/2)

	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return hex.EncodeToString(buf)[:length], nil
}

// GenerateDefaultRandomID returns a random ID of defaultIdLength lowercase hex
// characters. It returns an empty string when the random source fails, which
// practically never happens.
func GenerateDefaultRandomID() string {
	id, err := generateRandomID(defaultIdLength)
	if err != nil {
		return ""
	}

	return id
}

// defaultIdGenerator generates IDs of a fixed length.
type defaultIdGenerator struct {
	defaultIdLength int
}

// MakeID returns a random ID of the generator's length in lowercase hex
// characters. It returns an empty string when the random source fails, which
// practically never happens.
func (g *defaultIdGenerator) MakeID() string {
	id, err := generateRandomID(g.defaultIdLength)
	if err != nil {
		return ""
	}

	return id
}

// NewDefaultIdGenerator returns a generator of IDs of defaultIdLength
// lowercase hex characters.
func NewDefaultIdGenerator() *defaultIdGenerator {
	return &defaultIdGenerator{defaultIdLength: defaultIdLength}
}

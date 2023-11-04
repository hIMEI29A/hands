package helpers

import (
	"crypto/rand"
	"fmt"
)

const defaultIdLength = 24

// GenerateRandomID генерирует случайную строку длиной [length] символов
func GenerateRandomID(length int) string {
	id := make([]byte, length/2)
	rand.Read(id)

	return fmt.Sprintf("%x", id)
}

func GenerateDefaultRandomID() string {
	return GenerateRandomID(defaultIdLength)
}

type defaultIdGenerator struct {
	defaultIdLength int
}

func (g *defaultIdGenerator) MakeID() string {
	return GenerateRandomID(g.defaultIdLength)
}

func NewDefaultIdGenerator() *defaultIdGenerator {
	return &defaultIdGenerator{defaultIdLength: defaultIdLength}
}

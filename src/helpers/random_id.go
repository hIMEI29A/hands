package helpers

import (
	"crypto/rand"
	"fmt"
)

const defaultIdLength = 24

// generateRandomID генерирует случайную строку длиной [length] символов
func generateRandomID(length int) string {
	id := make([]byte, length/2)
	rand.Read(id)

	return fmt.Sprintf("%x", id)
}

func GenerateDefaultRandomID() string {
	return generateRandomID(defaultIdLength)
}

type defaultIdGenerator struct {
	defaultIdLength int
}

func (g *defaultIdGenerator) MakeID() string {
	return generateRandomID(g.defaultIdLength)
}

func NewDefaultIdGenerator() *defaultIdGenerator {
	return &defaultIdGenerator{defaultIdLength: defaultIdLength}
}

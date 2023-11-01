package helpers

import (
	"math/rand"
	"time"
)

func randomInt(num int) int {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)
	random := r.Intn(num)

	return random
}

func RandomInt(num int) int {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)
	random := r.Intn(num)

	return random
}

func GenerateRandomNumInRange(max int) int {
	return randomInt(max)
}

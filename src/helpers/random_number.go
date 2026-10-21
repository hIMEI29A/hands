package helpers

import (
	"math/rand"
	"sync"
	"time"
)

// sharedRandSource is the default *rand.Rand for GenerateRandomNumInRange.
// A single source is created once instead of on every call; it is guarded by a
// mutex because math/rand sources are not safe for concurrent use.
var (
	defaultRandMu sync.Mutex
	defaultRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// randomIntWithRand returns a random int in [0, max) using the given *rand.Rand.
// It returns 0 when max <= 0 instead of panicking (rand.Rand.Intn panics on
// non-positive input).
func randomIntWithRand(r *rand.Rand, max int) int {
	if max <= 0 {
		return 0
	}

	return r.Intn(max)
}

func randomInt(num int) int {
	defaultRandMu.Lock()
	defer defaultRandMu.Unlock()

	return randomIntWithRand(defaultRand, num)
}

// GenerateRandomNumInRange returns a random int in [0, max).
// It returns 0 when max <= 0.
func GenerateRandomNumInRange(max int) int {
	return randomInt(max)
}

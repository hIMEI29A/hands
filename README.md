# hands

[![CI](https://github.com/hIMEI29A/hands/actions/workflows/ci.yml/badge.svg?branch=dev)](https://github.com/hIMEI29A/hands/actions/workflows/ci.yml)

Texas Hold'em poker game library for Go.

## Current version

v0.3.1-alpha

## Usage

```go
package main

import (
	"fmt"
	"log"

	"hands/src/models"
)

func main() {
	// Strict card parsing: invalid input returns an error.
	card, err := models.ParseCard("10H")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(card) // 10H

	// Build and rank a hand; constructor errors on bad or duplicate cards.
	hand, err := models.NewHandFromStrings([]string{"AS", "KS", "QS", "JS", "10S"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(hand.Define())      // 9 (RoyalFlushHand)
	fmt.Println(hand.Compare(hand)) // 0 (equal)

	// Shuffle a fresh 52-card deck.
	deck := models.NewDeck().Shuffle()
	fmt.Println(deck.Len()) // 52
}
```

## Development

Local checks run through `make`:

```sh
make check      # gofmt, build, vet, tests with -race
make test-short # skip the exhaustive hand-category enumeration
make lint       # golangci-lint, if installed
```

CI runs the same build and test steps on Go `1.20.x` and `stable`; lint findings are reported but do not block merges.

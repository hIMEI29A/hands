# hands

Texas Hold'em poker game library for Go.

## Current version

v0.3.0-alpha

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

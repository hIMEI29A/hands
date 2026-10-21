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

## Betting

`Table` implements no-limit Texas Hold'em betting, including blinds,
turn order, all-ins and side pots.

```go
rules := models.DefaultTableRules(1, 2) // small blind, big blind

table, err := models.NewTableWithRules(
	"main", "cash", nil, models.CashTableType, 6, rules,
)
if err != nil {
	log.Fatal(err)
}

for _, name := range []string{"alice", "bob", "carol"} {
	if err := table.Register(models.NewPlayerWithDefaultID(name, 200)); err != nil {
		log.Fatal(err)
	}
}

// StartHand resets the table, posts blinds, deals pocket cards and opens
// preflop betting.
if _, err := table.StartHand(); err != nil {
	log.Fatal(err)
}

// Play every street, checking or calling for illustration.
for table.Phase() != models.PhaseShowdown {
	for table.Phase() == models.PhaseBetting {
		actor, err := table.CurrentActor()
		if err != nil {
			log.Fatal(err)
		}

		toCall, err := table.ToCall(actor.ID)
		if err != nil {
			log.Fatal(err)
		}

		action := models.Action{PlayerID: actor.ID, Type: models.ActionCheck}
		if toCall > 0 {
			action.Type = models.ActionCall
		}

		if _, err := table.Act(action); err != nil {
			log.Fatal(err)
		}
	}

	if table.NeedsNextStreet() {
		if _, err := table.NextStreet(); err != nil {
			log.Fatal(err)
		}
	}
}

results, err := table.Showdown()
if err != nil {
	log.Fatal(err)
}

for _, pot := range results {
	fmt.Println(pot.Amount, pot.Winners, pot.Payouts)
}
```

Bet and raise `Action.Amount` is the player's **total commitment on the
current street** ("raise to"), not the additional amount. There is no
separate all-in action: a call, bet or raise automatically becomes all-in
when it uses the rest of the player's stack.

`TableRules` controls the minimum players, blinds, minimum raise, optional
per-street raise cap and odd-chip rule. Only no-limit Hold'em is supported;
antes and straddles are not part of the current API.

## Development

Local checks run through `make`:

```sh
make check      # gofmt, build, vet, tests with -race
make test-short # skip the exhaustive hand-category enumeration
make lint       # golangci-lint, if installed
```

CI runs the same build and test steps on Go `1.20.x` and `stable`; lint findings are reported but do not block merges.

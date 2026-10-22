# hands

[![CI](https://github.com/hIMEI29A/hands/actions/workflows/ci.yml/badge.svg?branch=dev)](https://github.com/hIMEI29A/hands/actions/workflows/ci.yml)

Texas Hold'em poker game library for Go.

## Current version

v0.6.0-alpha

## Usage

```go
package main

import (
	"fmt"
	"log"

	"hands/src/engine"
)

func main() {
	// Strict card parsing: invalid input returns an error.
	card, err := engine.ParseCard("10H")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(card) // 10H

	// Build and rank a hand; constructor errors on bad or duplicate cards.
	hand, err := engine.NewHandFromStrings([]string{"AS", "KS", "QS", "JS", "10S"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(hand.Define())      // 9 (RoyalFlushHand)
	fmt.Println(hand.Compare(hand)) // 0 (equal)

	// Shuffle a fresh 52-card deck.
	deck := engine.NewDeck().Shuffle()
	fmt.Println(deck.Len()) // 52
}
```

## Betting

`Table` implements no-limit Texas Hold'em betting, including blinds, antes,
turn order, all-ins and side pots.

```go
rules := engine.DefaultTableRules(1, 2) // small blind, big blind
rules.Ante = 1                          // 1 chip from every active player
// rules.AnteMode = engine.AnteBigBlind // or only from the big blind

table, err := engine.NewTableWithRules(
	"main", "cash", nil, engine.CashTableType, 6, rules,
)
if err != nil {
	log.Fatal(err)
}

for _, name := range []string{"alice", "bob", "carol"} {
	if err := table.Register(engine.NewPlayerWithDefaultID(name, 200)); err != nil {
		log.Fatal(err)
	}
}

// StartHand resets the table, posts antes and blinds, deals pocket cards and
// opens preflop betting.
if _, err := table.StartHand(); err != nil {
	log.Fatal(err)
}

// Play every street, checking or calling for illustration.
for table.Phase() != engine.PhaseShowdown {
	for table.Phase() == engine.PhaseBetting {
		actor, err := table.CurrentActor()
		if err != nil {
			log.Fatal(err)
		}

		toCall, err := table.ToCall(actor.ID)
		if err != nil {
			log.Fatal(err)
		}

		action := engine.Action{PlayerID: actor.ID, Type: engine.ActionCheck}
		if toCall > 0 {
			action.Type = engine.ActionCall
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

Antes are optional and controlled by `TableRules.Ante` (0 disables them).
`TableRules.AnteMode` selects `engine.AntePerPlayer` (the default) so every
active player posts, or `engine.AnteBigBlind` so only the big blind posts.
Antes are **dead money** posted before the blinds: they go into the pot but
do not count toward `streetBet`, `currentBet`, `ToCall`, the minimum raise or
the raise cap. A short stack posts `min(ante, stack)` and is all-in for the
rest of the hand; it is still dealt pocket cards but cannot act and can only
win pots it is eligible for.

Bet and raise `Action.Amount` is the player's **total commitment on the
current street** ("raise to"), not the additional amount. There is no
separate all-in action: a call, bet or raise automatically becomes all-in
when it uses the rest of the player's stack.

`TableRules` controls the minimum players, blinds, antes (amount and
per-player vs big-blind mode), minimum raise, optional per-street raise cap
and odd-chip rule. Only no-limit Hold'em is supported; straddles are not part
of the current API.

## Tournament SNG

`src/tournament` adds a single-table freezeout sit-and-go on top of `Table`.
Blind levels advance by **completed hands**, not wall-clock time. A fixed
`Payout` table awards abstract prize units; prizes are not tournament chips.
With `hands/src/engine` and `hands/src/tournament` imported:

```go
rules := &tournament.TournamentRules{
	StartingStack: 1000,
	MinPlayers:    2, // default 2
	MaxPlayers:    6, // default 9
	Levels: []tournament.BlindLevel{
		{SmallBlind: 10, BigBlind: 20, Hands: 10},
		{SmallBlind: 15, BigBlind: 30, Ante: 5, Hands: 10},
	},
	Payouts: []tournament.Payout{
		{Place: 1, Amount: 60},
		{Place: 2, Amount: 30},
	},
}

tour, err := tournament.NewTournament("sng", "sng", nil, rules)
if err != nil {
	log.Fatal(err)
}

for _, name := range []string{"alice", "bob", "carol"} {
	if _, err := tour.Register(name); err != nil {
		log.Fatal(err)
	}
}

if err := tour.Start(); err != nil {
	log.Fatal(err)
}

for tour.State() != tournament.StateFinished {
	if err := tour.StartHand(); err != nil {
		log.Fatal(err)
	}

	for tour.Table().Phase() != engine.PhaseShowdown {
		if tour.Table().Phase() == engine.PhaseStreetComplete {
			if err := tour.NextStreet(); err != nil {
				log.Fatal(err)
			}
			continue
		}

		actor, err := tour.Table().CurrentActor()
		if err != nil {
			log.Fatal(err)
		}

		toCall, err := tour.Table().ToCall(actor.ID)
		if err != nil {
			log.Fatal(err)
		}

		action := engine.Action{PlayerID: actor.ID, Type: engine.ActionCheck}
		if toCall > 0 {
			action.Type = engine.ActionCall
		}

		if _, err := tour.Act(action); err != nil {
			log.Fatal(err)
		}
	}

	if _, err := tour.Showdown(); err != nil {
		log.Fatal(err)
	}
}

for _, standing := range tour.Standings() {
	fmt.Println(standing.Place, standing.Name, standing.Prize)
}
```

`BlindLevel.Hands` counts completed hands; a hand that starts on a level
finishes on that level. The last level repeats indefinitely. Payout places
must be contiguous starting at 1. A player eliminated with a larger
start-of-hand stack receives the better place; equal stacks share a place and
split the payout places they consume, with any remainder going clockwise from
the dealer. `Tournament.Table()` exposes the underlying table for read-only
inspection; use `Act`, `NextStreet` and `Showdown` to mutate tournament play.

Only single-table freezeout SNGs are implemented: no MTT, rebuy, re-entry,
late registration, bounties, hand-for-hand play or ICM. `Payout.Amount` is an
abstract `int` prize unit; buy-in and prize-pool accounting are out of scope.

## Development

Local checks run through `make`:

```sh
make check      # gofmt, build, vet, tests with -race
make test-short # skip the exhaustive hand-category enumeration
make lint       # golangci-lint, if installed
```

CI runs the same build and test steps on Go `1.20.x` and `stable`; lint findings are reported but do not block merges.

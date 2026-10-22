# hands

[![CI](https://github.com/hIMEI29A/hands/actions/workflows/ci.yml/badge.svg?branch=dev)](https://github.com/hIMEI29A/hands/actions/workflows/ci.yml)

Texas Hold'em poker game library for Go.

## Current version

v0.8.0-alpha

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

`src/tournament` adds a single-table sit-and-go on top of `Table`. The default
configuration is a freezeout; late registration, re-entry, add-ons, fixed
bounties and sit-out are opt-in. Blind levels advance by **completed hands**,
not wall-clock time. Prizes are abstract units, not tournament chips. With
`hands/src/engine` and `hands/src/tournament` imported:

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
must be contiguous starting at 1, fit within `MaxPlayers`, and must not exceed
the number of entries. A player eliminated with a larger start-of-hand stack
receives the better place; equal stacks share a place and split the payout
places they consume, with any remainder going clockwise from the dealer.
`Tournament.Table()` exposes the underlying table for read-only inspection;
use `Act`, `NextStreet` and `Showdown` to mutate tournament play.

### Buy-ins and prize pools

`BuyIn` is an abstract prize unit per entry and `Guarantee` is a minimum
pool. `Fee` is charged per entry but is not part of the pool. Bounties and
add-on fees are also outside the pool. `PrizePool()` returns
`max(BuyIn * entries + AddOnBuyIn * add-ons, Guarantee)`, so it grows during
registration and late registration. `EntryCost()` returns
`BuyIn + Fee + Bounty`, `AddOnCost()` returns `AddOnBuyIn + AddOnFee`, and
`FeesCollected()` returns `Fee * entries + AddOnFee * add-ons`. Entries include
late registrations and re-entries. Three payout modes are available:

- `PayoutModeFixed` (the default when `PayoutMode` is empty) pays the
  absolute `Payouts` table. Without a `BuyIn`, `Guarantee` or `AddOnBuyIn` it
  keeps the legacy behavior: payouts are abstract units and the prize pool is
  zero. With accounting enabled, `Start` rejects a payout total above the
  pool; a total below the pool leaves the remainder unawarded.
- `PayoutModePercent` pays `PercentPayouts`, whole percentages summing to 100.
- `PayoutModeWinnerTakeAll` pays the whole pool to first place.

Only the payout table for the selected mode may be set. `PayoutModePercent`
and `PayoutModeWinnerTakeAll` also require a positive `BuyIn` or `Guarantee`.

Percent prizes use integer arithmetic: every place is floored and the leftover
units go to the largest fractional remainders, with equal remainders favouring
the better place (the lower place number). A small pool may leave a place at
zero.

```go
rules := &tournament.TournamentRules{
	StartingStack:  1000,
	BuyIn:          100,
	Fee:            10,
	Guarantee:      500,
	Levels: []tournament.BlindLevel{
		{SmallBlind: 10, BigBlind: 20, Hands: 10},
	},
	PayoutMode:     tournament.PayoutModePercent,
	PercentPayouts: []tournament.PercentPayout{
		{Place: 1, Percent: 50},
		{Place: 2, Percent: 30},
		{Place: 3, Percent: 20},
	},
}
// PrizePool() is max(100 * entries, 500); the fee is not included.
```

### Late registration, re-entry and add-ons

`LateRegUntilLevel` opens late registration for the first N blind levels
(1-based, inclusive); 0 disables it. While the tournament is running,
`Register` seats a late entrant with the starting stack into a free seat or
the first busted seat, and fails when the table has no seat. Eliminated
entries already in the standings are shifted down one place so the final
field keeps unique places; re-entry does the same. `Level()` itself remains a
0-based index.

Re-entry (rebuy) requires `ReentryAllowed` and the same late-registration
window. `Reenter(previousPlayerID)` takes the ID of a busted entry and returns
a new player with a fresh starting stack, the same display name and a seat at
the busted seat or a free one. Only one entry in a re-entry chain may be
active at a time: the current entry must bust before another re-entry can be
bought. Every entry has its own player ID and standing row: the old entry
stays in `Standings` with its place, while the new entry starts from
`StartingStack`. `MaxReentries` caps re-entries per entry chain (0 means
unlimited), and `MaxPlayers` is a concurrent seat limit, not a total entry
limit.

`AddOnAfterLevel` (1-based; 0 disables) makes one add-on available to each
live entry after that level completes. The window is between the
level-completing `Showdown` and the first hand of the next level. `AddOn`
adds `AddOnChips` to the stack and records the add-on for accounting; each
entry may take at most one. The configured level must have a following level,
so there is no add-on window after the final level. `AddOnCost()` returns
`AddOnBuyIn + AddOnFee`.

```go
rules := &tournament.TournamentRules{
	StartingStack: 1000,
	BuyIn:         100,
	Fee:           10,
	Bounty:        5,
	Levels: []tournament.BlindLevel{
		{SmallBlind: 10, BigBlind: 20, Hands: 10},
		{SmallBlind: 15, BigBlind: 30, Hands: 10},
	},
	LateRegUntilLevel: 2,    // first two levels, 1-based; 0 disables
	ReentryAllowed:    true, // re-entry only while late registration is open
	MaxReentries:      1,    // per entry chain; 0 means unlimited
	AddOnAfterLevel:   1,
	AddOnBuyIn:        20,
	AddOnFee:          2,
	AddOnChips:        1000,
}

// While late registration is open, Register creates and seats a new entry.
late, err := tour.Register("dave")
if err != nil {
	log.Fatal(err)
}

// Reenter takes a busted entry's ID and returns the fresh entry.
player, err := tour.Reenter(bustedPlayerID)
if err != nil {
	log.Fatal(err)
}
fmt.Println(player.ID) // a new player ID, not bustedPlayerID

// AddOn is available only between the configured level and the next hand.
if err := tour.AddOn(late.ID); err != nil {
	log.Fatal(err)
}
```

All new rules fields are additive: their zero values preserve the original
freezeout SNG behavior.

### Bounties and sit-out

`Bounty` is a fixed bounty per entry. It is collected with the entry cost but
never funds the prize pool: `BountyPool()` returns `Bounty * entries`, and
`BountiesPaid()` returns the bounty units awarded so far. When an entry busts,
its bounty goes to the winners of the pot layer that eliminated it (the
largest non-refunded layer the busted player was eligible for). A split pot
splits the bounty evenly, with the remainder going clockwise from the dealer.
The last survivor receives their own bounty, plus any unclaimed bounty, when
the tournament finishes. `Standing.BountyWon` reports each entry's total.

`SitOut(playerID)` and `SitIn(playerID)` operate between hands on a live
entry. A sitting-out player keeps their chips but is not dealt into the next
hands, posts no blinds or antes and does not hold the button. The state is
exposed by `Standing.SittingOut` and `Player.IsSittingOut()`. Sitting in
requires a positive stack, and sitting-out players do not count toward the
table's minimum players.

Only single-table SNGs are implemented. Late registration is limited to the
configured level window, and re-entry requires that window to remain open.
There is no MTT (including MTT re-entry), no progressive or knockout bounty
(only a fixed per-entry bounty), no hand-for-hand play, no ICM or deal-making,
and no persisted hand history. `Payout.Amount`, `BuyIn`, `Fee`, `Bounty` and
`PrizePool()` are abstract `int` prize units, not tournament chips.

## Development

Local checks run through `make`:

```sh
make check      # gofmt, build, vet, tests with -race
make test-short # skip the exhaustive hand-category enumeration
make lint       # golangci-lint, if installed
```

CI runs the same build and test steps on Go `1.20.x` and `stable`; lint findings are reported but do not block merges.

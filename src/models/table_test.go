package models

import (
	"sort"
	"strings"
	"sync"
	"testing"
)

// newTestTable creates a table with a fixed ID, unlimited seats (0) and
// given blinds, so tests do not depend on randomness for reservations.
func newTestTable(name, tag string, maxPlayersNum, bb, sb int) *Table {
	return NewTable(name, tag, stubIDMaker{id: "table-" + name}, CasheTableType, maxPlayersNum, bb, sb)
}

func TestNewTableConstructors(t *testing.T) {
	t.Run("NewTable with stub idMaker", func(t *testing.T) {
		table := newTestTable("t1", "tag", -1, 100, 50)

		if table.ID != "table-t1" {
			t.Fatalf("ID = %q, want %q", table.ID, "table-t1")
		}

		if table.Pot == nil || table.Pot.Tag != "tag" {
			t.Fatalf("Pot = %+v, want pot with tag %q", table.Pot, "tag")
		}

		if len(table.Players) != 0 {
			t.Fatalf("len(Players) = %d, want 0", len(table.Players))
		}
	})

	t.Run("NewTable nil idMaker falls back", func(t *testing.T) {
		table := NewTable("t2", "tag", nil, CasheTableType, 9, 100, 50)

		if table.ID == "" {
			t.Fatal("NewTable with nil idMaker produced an empty ID")
		}
	})

	t.Run("NewTableWithDefaultId produces non-empty ID", func(t *testing.T) {
		table := NewTableWithDefaultId("t3", "tag", CasheTableType, 9, 100, 50)

		if table.ID == "" {
			t.Fatal("NewTableWithDefaultId produced an empty ID")
		}
	})

	t.Run("both constructors produce distinct IDs", func(t *testing.T) {
		a := NewTableWithDefaultId("t4-a", "tag", CasheTableType, 9, 100, 50)
		b := NewTableWithDefaultId("t4-b", "tag", CasheTableType, 9, 100, 50)

		if a.ID == b.ID {
			t.Fatalf("both tables share ID %q", a.ID)
		}
	})

	t.Run("deal funcs are set", func(t *testing.T) {
		table := newTestTable("t5", "tag", -1, 100, 50)

		funcs := table.GetDealFuncs()
		if len(funcs) != 4 {
			t.Fatalf("len(GetDealFuncs()) = %d, want 4", len(funcs))
		}
	})
}

func TestRegister(t *testing.T) {
	t.Run("success registers player and pot entry", func(t *testing.T) {
		table := newTestTable("reg-ok", "tag", -1, 100, 50)
		p := NewPlayer("alice", stubIDMaker{"alice"}, 1000)

		if err := table.Register(p); err != nil {
			t.Fatalf("Register: %v", err)
		}

		if got := len(table.Players); got != 1 {
			t.Fatalf("len(Players) = %d, want 1", got)
		}

		amt, err := table.Pot.GetPlayerAmount("alice")
		if err != nil {
			t.Fatalf("GetPlayerAmount: %v", err)
		}

		if amt != 0 {
			t.Fatalf("GetPlayerAmount = %d, want 0", amt)
		}
	})

	t.Run("duplicate ID rejected", func(t *testing.T) {
		table := newTestTable("reg-dup", "tag", -1, 100, 50)

		if err := table.Register(NewPlayer("a", stubIDMaker{"same"}, 100)); err != nil {
			t.Fatalf("first Register: %v", err)
		}

		if err := table.Register(NewPlayer("b", stubIDMaker{"same"}, 100)); err == nil {
			t.Fatal("duplicate registration succeeded")
		}

		if got := len(table.Players); got != 1 {
			t.Fatalf("len(Players) = %d, want 1", got)
		}
	})

	t.Run("nil player rejected", func(t *testing.T) {
		table := newTestTable("reg-nil", "tag", -1, 100, 50)

		if err := table.Register(nil); err == nil {
			t.Fatal("Register(nil) succeeded")
		}
	})

	t.Run("seat limit enforced", func(t *testing.T) {
		table := newTestTable("reg-limit", "tag", 2, 100, 50)

		if err := table.Register(NewPlayer("p1", stubIDMaker{"p1"}, 100)); err != nil {
			t.Fatalf("first Register: %v", err)
		}

		if err := table.Register(NewPlayer("p2", stubIDMaker{"p2"}, 100)); err != nil {
			t.Fatalf("second Register: %v", err)
		}

		if err := table.Register(NewPlayer("p3", stubIDMaker{"p3"}, 100)); err == nil {
			t.Fatal("third player accepted on a 2-seat table")
		}

		if got := len(table.Players); got != 2 {
			t.Fatalf("len(Players) = %d, want 2", got)
		}
	})

	t.Run("MaxPlayersNum 0 means unlimited", func(t *testing.T) {
		table := newTestTable("reg-unl", "tag", 0, 100, 50)

		for _, id := range []string{"a", "b", "c", "d"} {
			if err := table.Register(NewPlayer(id, stubIDMaker{id}, 100)); err != nil {
				t.Fatalf("Register(%s): %v", id, err)
			}
		}

		if got := len(table.Players); got != 4 {
			t.Fatalf("len(Players) = %d, want 4", got)
		}
	})
}

func TestRegisterConcurrent(t *testing.T) {
	table := newTestTable("reg-race", "tag", 0, 100, 50)

	const goroutines = 2
	const perGoroutine = 5

	var wg sync.WaitGroup
	errs := make([][]string, goroutines)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)

		go func(g int) {
			defer wg.Done()

			for i := 0; i < perGoroutine; i++ {
				id := makeID("p", g, i)
				if err := table.Register(NewPlayer(id, stubIDMaker{id}, 100)); err != nil {
					errs[g] = append(errs[g], err.Error())
				}
			}
		}(g)
	}

	wg.Wait()

	for g := range errs {
		if len(errs[g]) != 0 {
			t.Fatalf("goroutine %d got errors: %v", g, errs[g])
		}
	}

	if got := len(table.Players); got != goroutines*perGoroutine {
		t.Fatalf("len(Players) = %d, want %d (lost or duplicated registrations)", got, goroutines*perGoroutine)
	}

	seen := map[string]int{}
	for _, p := range table.Players {
		seen[p.ID]++
	}

	for g := 0; g < goroutines; g++ {
		for i := 0; i < perGoroutine; i++ {
			id := makeID("p", g, i)

			if seen[id] != 1 {
				t.Fatalf("player %s registered %d times, want 1", id, seen[id])
			}

			if _, err := table.Pot.GetPlayerAmount(id); err != nil {
				t.Fatalf("GetPlayerAmount(%s): %v", id, err)
			}
		}
	}
}

func TestRegisterConcurrentSameID(t *testing.T) {
	table := newTestTable("reg-race-same", "tag", 0, 100, 50)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0

	for i := 0; i < 4; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := table.Register(NewPlayer("dup", stubIDMaker{"dup"}, 100)); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if successes != 1 {
		t.Fatalf("successful registrations = %d, want 1", successes)
	}

	if got := len(table.Players); got != 1 {
		t.Fatalf("len(Players) = %d, want 1", got)
	}
}

func makeID(prefix string, g, i int) string {
	return prefix + string(rune('0'+g)) + "-" + string(rune('0'+i))
}

func TestPositions(t *testing.T) {
	setup := func(t *testing.T, names []string) *Table {
		table := newTestTable("pos", "tag", 0, 100, 50)

		for _, n := range names {
			if err := table.Register(NewPlayer(n, stubIDMaker{n}, 1000)); err != nil {
				t.Fatalf("Register(%s): %v", n, err)
			}
		}

		return table
	}

	for _, names := range [][]string{{"a", "b"}, {"a", "b", "c"}, {"a", "b", "c", "d"}} {
		n := len(names)

		for _, dealer := range []int{0, n - 1} {
			table := setup(t, names)
			table.Dealer = dealer

			firstP, err := table.GetFirstPosition()
			if err != nil {
				t.Fatalf("%d players, dealer %d: GetFirstPosition: %v", n, dealer, err)
			}

			wantFirst := names[(dealer+1)%n]
			if firstP.ID != wantFirst {
				t.Fatalf("%d players, dealer %d: first = %q, want %q", n, dealer, firstP.ID, wantFirst)
			}

			secondP, err := table.GetSecondPosition()
			if err != nil {
				t.Fatalf("%d players, dealer %d: GetSecondPosition: %v", n, dealer, err)
			}

			wantSecond := names[(dealer+2)%n]
			if secondP.ID != wantSecond {
				t.Fatalf("%d players, dealer %d: second = %q, want %q", n, dealer, secondP.ID, wantSecond)
			}

			d, err := table.GetDealer()
			if err != nil {
				t.Fatalf("%d players, dealer %d: GetDealer: %v", n, dealer, err)
			}

			if d.ID != names[dealer] {
				t.Fatalf("%d players: dealer = %q, want %q", n, d.ID, names[dealer])
			}
		}
	}
}

func TestPositionsEmptyTableReturnsError(t *testing.T) {
	table := newTestTable("pos-empty", "tag", 0, 100, 50)

	if _, err := table.GetFirstPosition(); err == nil {
		t.Fatal("GetFirstPosition on empty table returned no error")
	}

	if _, err := table.GetSecondPosition(); err == nil {
		t.Fatal("GetSecondPosition on empty table returned no error")
	}

	if _, err := table.GetDealer(); err == nil {
		t.Fatal("GetDealer on empty table returned no error")
	}
}

// setDealerFixed makes dealer rotation deterministic: disables the random
// first pick and sets a known dealer index.
func setDealerFixed(t *testing.T, table *Table, dealer int) {
	t.Helper()

	table.isDealerInactive = false
	table.Dealer = dealer
}

func TestNextDealer(t *testing.T) {
	t.Run("first call picks a valid index", func(t *testing.T) {
		table := setupPlayers(t, "next-1", 3)

		d := table.NextDealer()

		if d < 0 || d >= len(table.Players) {
			t.Fatalf("NextDealer() = %d, want an index in [0, %d)", d, len(table.Players))
		}
	})

	t.Run("rotates and wraps", func(t *testing.T) {
		table := setupPlayers(t, "next-2", 3)
		setDealerFixed(t, table, 0)

		if got := table.NextDealer(); got != 1 {
			t.Fatalf("NextDealer() = %d, want 1", got)
		}

		if got := table.NextDealer(); got != 2 {
			t.Fatalf("NextDealer() = %d, want 2", got)
		}

		if got := table.NextDealer(); got != 0 {
			t.Fatalf("NextDealer() = %d, want 0 (wrap)", got)
		}
	})

	t.Run("wraps from the last seat", func(t *testing.T) {
		table := setupPlayers(t, "next-3", 3)
		setDealerFixed(t, table, 2)

		if got := table.NextDealer(); got != 0 {
			t.Fatalf("NextDealer() = %d, want 0", got)
		}
	})

	t.Run("empty table returns current dealer", func(t *testing.T) {
		table := newTestTable("next-4", "tag", 0, 100, 50)
		table.Dealer = 7

		if got := table.NextDealer(); got != 7 {
			t.Fatalf("NextDealer() on empty table = %d, want 7 (unchanged)", got)
		}
	})
}

func TestStartHand(t *testing.T) {
	t.Run("requires at least 2 players", func(t *testing.T) {
		table := setupPlayers(t, "start-1", 1)

		if _, err := table.StartHand(); err == nil {
			t.Fatal("StartHand with 1 player succeeded")
		}
	})

	t.Run("resets state and shuffles deck", func(t *testing.T) {
		table := setupPlayers(t, "start-2", 3)
		p1, p2, p3 := table.Players[0], table.Players[1], table.Players[2]

		// Dirty state from a previous hand.
		board := mustCards(t, "2H", "3D", "5S", "9H", "KD")
		table.Board = append(table.Board, board...)

		if err := p1.AddCard(mustCards(t, "AS")[0]); err != nil {
			t.Fatalf("AddCard: %v", err)
		}

		if err := p2.AddCard(mustCards(t, "QD")[0]); err != nil {
			t.Fatalf("AddCard: %v", err)
		}

		// p2 falls and keeps only 1 pocket card.
		if _, err := p2.Fall(NewBet(0, 0)); err != nil {
			t.Fatalf("Fall: %v", err)
		}

		if _, err := table.Pot.AddPlayerBet(Chips(30), p1.ID); err != nil {
			t.Fatalf("AddPlayerBet: %v", err)
		}

		table.isDealerInactive = false

		if _, err := table.StartHand(); err != nil {
			t.Fatalf("StartHand: %v", err)
		}

		if got := len(table.Board); got != 0 {
			t.Fatalf("len(Board) = %d, want 0", got)
		}

		for _, p := range []*Player{p1, p2, p3} {
			if !p.Active {
				t.Fatalf("player %s is inactive after StartHand", p.ID)
			}

			if got := len(p.GetPocketCards()); got != 0 {
				t.Fatalf("player %s has %d pocket cards after StartHand, want 0", p.ID, got)
			}
		}

		if got := table.Pot.TotalChipsNum; got != 0 {
			t.Fatalf("Pot.TotalChipsNum = %d, want 0", got)
		}

		for _, p := range []*Player{p1, p2, p3} {
			amt, err := table.Pot.GetPlayerAmount(p.ID)
			if err != nil {
				t.Fatalf("GetPlayerAmount(%s): %v", p.ID, err)
			}

			if amt != 0 {
				t.Fatalf("GetPlayerAmount(%s) = %d, want 0", p.ID, amt)
			}
		}

		if got := table.deck.Len(); got != DeckLength {
			t.Fatalf("deck length after StartHand = %d, want %d", got, DeckLength)
		}

		if table.isDealerInactive {
			t.Fatal("dealer was not activated by StartHand")
		}

		if table.Dealer < 0 || table.Dealer >= len(table.Players) {
			t.Fatalf("Dealer = %d, want a valid index", table.Dealer)
		}
	})
}

func setupPlayers(t *testing.T, tag string, count int) *Table {
	t.Helper()

	table := newTestTable(tag, "tag", 0, 100, 50)

	for i := 0; i < count; i++ {
		id := setupPlayerID(i)

		if err := table.Register(NewPlayer(id, stubIDMaker{id}, 1000)); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	return table
}

func setupPlayerID(i int) string {
	return "player-" + string(rune('a'+i))
}

func TestDealStreets(t *testing.T) {
	table := setupPlayers(t, "deal-flow", 3)

	if _, err := table.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	// Streets out of order fail before the missing ones are dealt.
	if _, err := table.Turn(); err == nil {
		t.Fatal("Turn before Flop succeeded")
	}

	if _, err := table.River(); err == nil {
		t.Fatal("River before Flop succeeded")
	}

	if _, err := table.PreFlop(); err != nil {
		t.Fatalf("PreFlop: %v", err)
	}

	for _, p := range table.Players {
		if got := len(p.GetPocketCards()); got != PocketSize {
			t.Fatalf("player %s has %d pocket cards after PreFlop, want %d", p.ID, got, PocketSize)
		}
	}

	if got := len(table.Board); got != 0 {
		t.Fatalf("len(Board) after PreFlop = %d, want 0", got)
	}

	// PreFlop again is invalid: pockets are filled.
	if _, err := table.PreFlop(); err == nil {
		t.Fatal("second PreFlop succeeded")
	}

	if _, err := table.Flop(); err != nil {
		t.Fatalf("Flop: %v", err)
	}

	if got := len(table.Board); got != CardsOnFlopNumber {
		t.Fatalf("len(Board) after Flop = %d, want %d", got, CardsOnFlopNumber)
	}

	// Flop twice is invalid: board is not empty.
	if _, err := table.Flop(); err == nil {
		t.Fatal("second Flop succeeded")
	}

	if _, err := table.Turn(); err != nil {
		t.Fatalf("Turn: %v", err)
	}

	if got := len(table.Board); got != 4 {
		t.Fatalf("len(Board) after Turn = %d, want 4", got)
	}

	// River right after Flop is invalid: board has 3 cards, not 4.
	boardState := len(table.Board)
	table.Board = table.Board[:3]

	if _, err := table.River(); err == nil {
		t.Fatal("River with a 3-card board succeeded")
	}

	table.Board = table.Board[:boardState]

	if _, err := table.River(); err != nil {
		t.Fatalf("River: %v", err)
	}

	if got := len(table.Board); got != 5 {
		t.Fatalf("len(Board) after River = %d, want 5", got)
	}
}

func TestDealingUnshuffledDeckErrors(t *testing.T) {
	table := setupPlayers(t, "deal-raw", 2)

	// No StartHand, so the deck is empty and nothing can be dealt.
	if _, err := table.PreFlop(); err == nil {
		t.Fatal("PreFlop on an unshuffled deck succeeded")
	}

	if _, err := table.Flop(); err == nil {
		t.Fatal("Flop on an unshuffled deck succeeded")
	}
}

func TestBlinds(t *testing.T) {
	t.Run("posts small and big blind", func(t *testing.T) {
		table := newTestTable("bl-1", "tag", 0, 20, 10)

		p0 := NewPlayer("p0", stubIDMaker{"p0"}, 1000)
		p1 := NewPlayer("p1", stubIDMaker{"p1"}, 1000)
		p2 := NewPlayer("p2", stubIDMaker{"p2"}, 1000)

		for _, p := range []*Player{p0, p1, p2} {
			if err := table.Register(p); err != nil {
				t.Fatalf("Register: %v", err)
			}
		}

		table.Dealer = 0
		table.isDealerInactive = false

		if _, err := table.Blinds(); err != nil {
			t.Fatalf("Blinds: %v", err)
		}

		// Small blind = seat after the dealer, big blind = the one after it.
		amt, err := table.Pot.GetPlayerAmount(p1.ID)
		if err != nil {
			t.Fatalf("GetPlayerAmount: %v", err)
		}

		if amt != 10 {
			t.Fatalf("small blind contribution = %d, want 10", amt)
		}

		if got := p1.GetCurrentChipsAmount(); got != Chips(990) {
			t.Fatalf("small blind stack = %d, want 990", got)
		}

		amt, err = table.Pot.GetPlayerAmount(p2.ID)
		if err != nil {
			t.Fatalf("GetPlayerAmount: %v", err)
		}

		if amt != 20 {
			t.Fatalf("big blind contribution = %d, want 20", amt)
		}

		if got := p2.GetCurrentChipsAmount(); got != Chips(980) {
			t.Fatalf("big blind stack = %d, want 980", got)
		}

		if got := table.Pot.TotalChipsNum; got != Chips(30) {
			t.Fatalf("pot total = %d, want 30", got)
		}

		// The dealer did not pay anything.
		amt, err = table.Pot.GetPlayerAmount(p0.ID)
		if err != nil {
			t.Fatalf("GetPlayerAmount: %v", err)
		}

		if amt != 0 {
			t.Fatalf("dealer contribution = %d, want 0", amt)
		}
	})

	t.Run("fewer than 2 players errors", func(t *testing.T) {
		table := setupPlayers(t, "bl-2", 1)

		if _, err := table.Blinds(); err == nil {
			t.Fatal("Blinds with 1 player succeeded")
		}
	})

	t.Run("inactive players are skipped", func(t *testing.T) {
		table := newTestTable("bl-3", "tag", 0, 20, 10)

		p0 := NewPlayer("p0", stubIDMaker{"p0"}, 1000)
		p1 := NewPlayer("p1", stubIDMaker{"p1"}, 1000)
		p2 := NewPlayer("p2", stubIDMaker{"p2"}, 1000)

		for _, p := range []*Player{p0, p1, p2} {
			if err := table.Register(p); err != nil {
				t.Fatalf("Register: %v", err)
			}
		}

		table.Dealer = 0
		table.isDealerInactive = false

		// Seat after the dealer folds; blinds skip to the next seats.
		if _, err := p1.Fall(NewBet(0, 0)); err != nil {
			t.Fatalf("Fall: %v", err)
		}

		if _, err := table.Blinds(); err != nil {
			t.Fatalf("Blinds: %v", err)
		}

		amt, err := table.Pot.GetPlayerAmount(p1.ID)
		if err != nil {
			t.Fatalf("GetPlayerAmount: %v", err)
		}

		if amt != 0 {
			t.Fatalf("inactive player was charged %d, want 0", amt)
		}

		if got := p1.GetCurrentChipsAmount(); got != Chips(1000) {
			t.Fatalf("inactive player stack = %d, want 1000", got)
		}

		// p2 posts the small blind, p0 the big blind.
		amt, err = table.Pot.GetPlayerAmount(p2.ID)
		if err != nil {
			t.Fatalf("GetPlayerAmount: %v", err)
		}

		if amt != 10 {
			t.Fatalf("small blind contribution = %d, want 10", amt)
		}

		amt, err = table.Pot.GetPlayerAmount(p0.ID)
		if err != nil {
			t.Fatalf("GetPlayerAmount: %v", err)
		}

		if amt != 20 {
			t.Fatalf("big blind contribution = %d, want 20", amt)
		}
	})
}

func TestResolveWinnerSingleWinner(t *testing.T) {
	table := setupPlayers(t, "win-single", 3)

	// Board: 2S 3D 5H 9H KC.
	setBoard(t, table, "2S", "3D", "5H", "9H", "KC")

	// player-a holds three nines and beats the other holdings.
	setPocket(t, table, "player-a", "9S", "9D")
	setPocket(t, table, "player-b", "2C", "3C")
	setPocket(t, table, "player-c", "4C", "5C")

	ids, err := table.ResolveWinner()
	if err != nil {
		t.Fatalf("ResolveWinner: %v", err)
	}

	if len(ids) != 1 {
		t.Fatalf("ResolveWinner returned %v, want %v", ids, []string{"player-a"})
	}

	if ids[0] != "player-a" {
		t.Fatalf("ResolveWinner winner = %q, want %q", ids[0], "player-a")
	}
}

func TestResolveWinnerSplitPot(t *testing.T) {
	table := setupPlayers(t, "win-split", 3)

	// Board: 2H 3D 5S 9H AD.
	setBoard(t, table, "2H", "3D", "5S", "9H", "AD")

	// Both players hold the best two pair (aces and kings): board AD
	// plus AK in the pocket for both.
	setPocket(t, table, "player-a", "AS", "KD")
	setPocket(t, table, "player-b", "AH", "KH")
	setPocket(t, table, "player-c", "2C", "7D")

	ids, err := table.ResolveWinner()
	if err != nil {
		t.Fatalf("ResolveWinner: %v", err)
	}

	if len(ids) != 2 {
		t.Fatalf("ResolveWinner returned %v, want 2 winners", ids)
	}

	if !sort.StringsAreSorted(ids) {
		t.Fatalf("ResolveWinner returned %v, want sorted output", ids)
	}

	if ids[0] != "player-a" || ids[1] != "player-b" {
		t.Fatalf("ResolveWinner returned %v, want [%s %s]", ids, "player-a", "player-b")
	}
}

func TestResolveWinnerIgnoresFallenPlayers(t *testing.T) {
	table := setupPlayers(t, "win-fallen", 2)

	setBoard(t, table, "2H", "3D", "5S", "9H", "KD")

	setPocket(t, table, "player-a", "AS", "AD")
	setPocket(t, table, "player-b", "2C", "3C")

	// player-b has the worse hand but folds.
	if _, err := table.Players[1].Fall(NewBet(0, 0)); err != nil {
		t.Fatalf("Fall: %v", err)
	}

	ids, err := table.ResolveWinner()
	if err != nil {
		t.Fatalf("ResolveWinner: %v", err)
	}

	if len(ids) != 1 || ids[0] != "player-a" {
		t.Fatalf("ResolveWinner returned %v, want [player-a] (only active players)", ids)
	}
}

func TestResolveWinnerErrors(t *testing.T) {
	t.Run("incomplete board", func(t *testing.T) {
		table := setupPlayers(t, "win-err-board", 2)
		setBoard(t, table, "2H", "3D", "5S", "9H")

		if _, err := table.ResolveWinner(); err == nil {
			t.Fatal("ResolveWinner with a 4-card board succeeded")
		}
	})

	t.Run("no players", func(t *testing.T) {
		table := newTestTable("win-err-empty", "tag", 0, 100, 50)
		setBoard(t, table, "2H", "3D", "5S", "9H", "KD")

		if _, err := table.ResolveWinner(); err == nil {
			t.Fatal("ResolveWinner with no players succeeded")
		}
	})

	t.Run("player with fewer than 2 pocket cards", func(t *testing.T) {
		table := setupPlayers(t, "win-err-pockets", 2)
		setBoard(t, table, "2H", "3D", "5S", "9H", "KD")

		setPocket(t, table, "player-a", "AS", "AD")
		setPocket(t, table, "player-b", "2C")

		if _, err := table.ResolveWinner(); err == nil {
			t.Fatal("ResolveWinner with a 1-card pocket succeeded")
		}
	})
}

func TestResolveWinnerDoesNotMutateBoard(t *testing.T) {
	table := setupPlayers(t, "win-board", 2)

	setBoard(t, table, "2H", "3D", "5S", "9H", "KD")

	setPocket(t, table, "player-a", "AS", "AD")
	setPocket(t, table, "player-b", "2C", "3C")

	before := boardString(t, table)

	if _, err := table.ResolveWinner(); err != nil {
		t.Fatalf("ResolveWinner: %v", err)
	}

	after := boardString(t, table)

	if before != after {
		t.Fatalf("board changed: before %s, after %s", before, after)
	}
}

func setBoard(t *testing.T, table *Table, ss ...string) {
	t.Helper()

	table.m.Lock()
	defer table.m.Unlock()

	table.Board = mustCards(t, ss...)
}

// setPocket deals the given cards to the player's pocket. Accepting any
// count (including fewer than 2) lets tests build invalid states on
// purpose, e.g. a player with a single pocket card.
func setPocket(t *testing.T, table *Table, playerID string, ss ...string) {
	t.Helper()

	p := table.GetPlayerByID(playerID)
	if p == nil {
		t.Fatalf("player %q not found", playerID)
	}

	for _, c := range mustCards(t, ss...) {
		p.mu.Lock()
		p.pocketCards = append(p.pocketCards, c)
		p.mu.Unlock()
	}
}

func boardString(t *testing.T, table *Table) string {
	t.Helper()

	table.m.RLock()
	defer table.m.RUnlock()

	out := make([]string, 0, len(table.Board))

	for _, c := range table.Board {
		out = append(out, c.String())
	}

	return strings.Join(out, ",")
}

func TestBlindsHeadsUp(t *testing.T) {
	table := newTestTable("bl-hu", "tag", 0, 20, 10)

	dealer := NewPlayer("dealer", stubIDMaker{"dealer"}, 1000)
	other := NewPlayer("other", stubIDMaker{"other"}, 1000)

	for _, p := range []*Player{dealer, other} {
		if err := table.Register(p); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	table.Dealer = 0
	table.isDealerInactive = false

	if _, err := table.Blinds(); err != nil {
		t.Fatalf("Blinds: %v", err)
	}

	// Heads-up: the dealer posts the small blind, the other player the big one.
	dealerAmount, err := table.Pot.GetPlayerAmount(dealer.ID)
	if err != nil {
		t.Fatalf("GetPlayerAmount: %v", err)
	}

	if dealerAmount != 10 {
		t.Errorf("dealer (small blind) contribution = %d, want 10", dealerAmount)
	}

	if got := dealer.GetCurrentChipsAmount(); got != Chips(990) {
		t.Errorf("dealer stack = %d, want 990", got)
	}

	otherAmount, err := table.Pot.GetPlayerAmount(other.ID)
	if err != nil {
		t.Fatalf("GetPlayerAmount: %v", err)
	}

	if otherAmount != 20 {
		t.Errorf("other (big blind) contribution = %d, want 20", otherAmount)
	}

	if got := other.GetCurrentChipsAmount(); got != Chips(980) {
		t.Errorf("other stack = %d, want 980", got)
	}
}

func TestBlindsRequiresTwoActivePlayers(t *testing.T) {
	table := newTestTable("bl-one-active", "tag", 0, 20, 10)

	p0 := NewPlayer("p0", stubIDMaker{"p0"}, 1000)
	p1 := NewPlayer("p1", stubIDMaker{"p1"}, 1000)

	for _, p := range []*Player{p0, p1} {
		if err := table.Register(p); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	if _, err := p1.Fall(NewBet(0, 0)); err != nil {
		t.Fatalf("Fall: %v", err)
	}

	if _, err := table.Blinds(); err == nil {
		t.Fatal("Blinds with a single active player succeeded")
	}
}

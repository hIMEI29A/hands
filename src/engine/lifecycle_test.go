package engine

import (
	"fmt"
	"testing"
)

// newLifecycleTable builds a waiting table with deterministic IDs and the
// given stacks. No hand is started.
func newLifecycleTable(t *testing.T, name string, maxPlayersNum int, stacks []Chips) *Table {
	t.Helper()

	table := newTestTable(name, "tag", maxPlayersNum, 10, 5)

	for i, stack := range stacks {
		id := fmt.Sprintf("p%d", i)
		if err := table.Register(NewPlayer(id, stubIDMaker{id}, stack)); err != nil {
			t.Fatalf("Register(%s): %v", id, err)
		}
	}

	return table
}

func TestSittingOutPlayerSkipsTheHand(t *testing.T) {
	rules := DefaultTableRules(5, 10)
	rules.Ante = 2

	table, err := NewTableWithRules("lifecycle-sitout", "tag", stubIDMaker{"lifecycle-sitout"}, CashTableType, 0, rules)
	if err != nil {
		t.Fatalf("NewTableWithRules: %v", err)
	}

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("p%d", i)
		if err := table.Register(NewPlayer(id, stubIDMaker{id}, 100)); err != nil {
			t.Fatalf("Register(%s): %v", id, err)
		}
	}

	// The dealer button starts at p0 and moves to the next active seat,
	// which must skip the sitting-out p1.
	table.isDealerInactive = false
	table.Dealer = 0

	if err := table.SitOut("p1"); err != nil {
		t.Fatalf("SitOut: %v", err)
	}

	if _, err := table.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	sittingOut := table.GetPlayerByID("p1")

	if !sittingOut.IsSittingOut() {
		t.Fatal("p1 is not marked as sitting out")
	}

	if sittingOut.Active {
		t.Fatal("sitting-out player is active")
	}

	if got := sittingOut.GetCurrentChipsAmount(); got != 100 {
		t.Fatalf("sitting-out player chips = %d, want 100", got)
	}

	if got := len(sittingOut.GetPocketCards()); got != 0 {
		t.Fatalf("sitting-out player has %d pocket cards, want 0", got)
	}

	if got := sittingOut.GetStreetBet(); got != 0 {
		t.Fatalf("sitting-out player street bet = %d, want 0", got)
	}

	if dealer, err := table.GetDealer(); err != nil || dealer.ID != "p2" {
		t.Fatalf("GetDealer() = %v, %v; want p2 (p1 is sitting out)", dealer, err)
	}

	// Two active players: p2 posts the small blind, p0 the big blind and
	// both post the per-player ante of 2. The sit-out pays nothing.
	wantPot := 2*Chips(2) + 5 + 10
	if got := table.Pot.TotalChipsNum; got != wantPot {
		t.Fatalf("pot total = %d, want %d", got, wantPot)
	}

	if got := table.GetPlayerByID("p0").GetCurrentChipsAmount(); got != 88 {
		t.Fatalf("p0 chips = %d, want 88", got)
	}

	if got := table.GetPlayerByID("p2").GetCurrentChipsAmount(); got != 93 {
		t.Fatalf("p2 chips = %d, want 93", got)
	}

	for _, id := range []string{"p0", "p2"} {
		if got := len(table.GetPlayerByID(id).GetPocketCards()); got != PocketSize {
			t.Fatalf("%s has %d pocket cards, want %d", id, got, PocketSize)
		}
	}

	if actor, err := table.CurrentActor(); err == nil && actor.ID == "p1" {
		t.Fatal("sitting-out player is the current actor")
	}
}

func TestSitInBeforeNextHandRestoresPlayer(t *testing.T) {
	table := newLifecycleTable(t, "lifecycle-sitin", 0, []Chips{100, 100, 100})

	table.isDealerInactive = false
	table.Dealer = 0

	if err := table.SitOut("p1"); err != nil {
		t.Fatalf("SitOut: %v", err)
	}

	if _, err := table.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	playHandToShowdown(t, table)

	if _, err := table.Showdown(); err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	p := table.GetPlayerByID("p1")

	if err := table.SitIn("p1"); err != nil {
		t.Fatalf("SitIn: %v", err)
	}

	if p.IsSittingOut() {
		t.Fatal("p1 is still sitting out after SitIn")
	}

	if !p.Active {
		t.Fatal("p1 is inactive after SitIn")
	}

	if _, err := table.StartHand(); err != nil {
		t.Fatalf("StartHand after SitIn: %v", err)
	}

	if !p.Active {
		t.Fatal("p1 is inactive in the new hand")
	}

	if got := len(p.GetPocketCards()); got != PocketSize {
		t.Fatalf("p1 has %d pocket cards after SitIn, want %d", got, PocketSize)
	}
}

func TestSittingOutDoesNotCountTowardMinPlayers(t *testing.T) {
	t.Run("two funded players, one sits out", func(t *testing.T) {
		table := newHeadsUpWaitingTable(t, "lifecycle-min-two", 100, 5, 10)

		if err := table.SitOut("p0"); err != nil {
			t.Fatalf("SitOut: %v", err)
		}

		if _, err := table.StartHand(); err == nil {
			t.Fatal("StartHand succeeded with one funded player after a sit-out")
		}

		if table.Phase() != PhaseWaiting {
			t.Fatalf("phase = %s, want %s", table.Phase(), PhaseWaiting)
		}
	})

	t.Run("three funded players, one sits out", func(t *testing.T) {
		table := newLifecycleTable(t, "lifecycle-min-three", 0, []Chips{100, 100, 100})

		if err := table.SitOut("p1"); err != nil {
			t.Fatalf("SitOut: %v", err)
		}

		if _, err := table.StartHand(); err != nil {
			t.Fatalf("StartHand: %v", err)
		}

		if got := table.CurrentPlayersNum; got != 2 {
			t.Fatalf("CurrentPlayersNum = %d, want 2", got)
		}
	})
}

func TestReplacePlayer(t *testing.T) {
	t.Run("preserves seat index and dealer", func(t *testing.T) {
		table := newLifecycleTable(t, "lifecycle-replace", 0, []Chips{100, 0, 100})

		table.isDealerInactive = false
		table.Dealer = 1

		replacement := NewPlayer("p3", stubIDMaker{"p3"}, 50)

		// Leftover hand state must be cleared before seating.
		if err := replacement.AddCard(mustCards(t, "AS")[0]); err != nil {
			t.Fatalf("AddCard: %v", err)
		}

		if _, err := replacement.commit(5); err != nil {
			t.Fatalf("commit: %v", err)
		}

		replacement.Active = false

		if err := table.ReplacePlayer("p1", replacement); err != nil {
			t.Fatalf("ReplacePlayer: %v", err)
		}

		if got := len(table.Players); got != 3 {
			t.Fatalf("len(Players) = %d, want 3", got)
		}

		if got := table.Players[1].ID; got != "p3" {
			t.Fatalf("Players[1].ID = %q, want p3", got)
		}

		if got := table.Dealer; got != 1 {
			t.Fatalf("Dealer = %d, want 1", got)
		}

		if dealer, err := table.GetDealer(); err != nil || dealer.ID != "p3" {
			t.Fatalf("GetDealer() = %v, %v; want p3", dealer, err)
		}

		if table.GetPlayerByID("p1") != nil {
			t.Fatal("replaced player is still at the table")
		}

		if !replacement.Active {
			t.Fatal("replacement is inactive")
		}

		if got := len(replacement.GetPocketCards()); got != 0 {
			t.Fatalf("replacement has %d pocket cards, want 0", got)
		}

		if got := replacement.GetStreetBet(); got != 0 {
			t.Fatalf("replacement street bet = %d, want 0", got)
		}

		if got := replacement.GetCurrentChipsAmount(); got != 45 {
			t.Fatalf("replacement chips = %d, want 45", got)
		}
	})

	t.Run("rejects mid-hand calls", func(t *testing.T) {
		table := newEngineTable(t, []Chips{100, 100}, 5, 10)

		if err := table.ReplacePlayer("p0", NewPlayer("new", stubIDMaker{"new"}, 100)); err == nil {
			t.Fatal("ReplacePlayer succeeded during betting")
		}

		playHandToShowdown(t, table)

		if err := table.ReplacePlayer("p0", NewPlayer("new", stubIDMaker{"new"}, 100)); err == nil {
			t.Fatal("ReplacePlayer succeeded during showdown")
		}
	})

	t.Run("rejects funded old player", func(t *testing.T) {
		table := newLifecycleTable(t, "lifecycle-replace-funded", 0, []Chips{100, 0})

		if err := table.ReplacePlayer("p0", NewPlayer("new", stubIDMaker{"new"}, 100)); err == nil {
			t.Fatal("ReplacePlayer replaced a funded player")
		}

		if got := table.Players[0].ID; got != "p0" {
			t.Fatalf("Players[0].ID = %q, want p0", got)
		}
	})

	t.Run("rejects invalid new players", func(t *testing.T) {
		tests := []struct {
			name   string
			oldID  string
			player *Player
		}{
			{"nil player", "p1", nil},
			{"duplicate ID", "p1", NewPlayer("p0", stubIDMaker{"p0"}, 100)},
			{"same as old ID", "p1", NewPlayer("p1", stubIDMaker{"p1"}, 100)},
			{"empty ID", "p1", NewPlayer("", stubIDMaker{""}, 100)},
			{"no stack", "p1", NewPlayer("new", stubIDMaker{"new"}, 0)},
			{"unknown old ID", "missing", NewPlayer("new", stubIDMaker{"new"}, 100)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				table := newLifecycleTable(t, "lifecycle-replace-invalid", 0, []Chips{100, 0})

				if err := table.ReplacePlayer(tt.oldID, tt.player); err == nil {
					t.Fatalf("ReplacePlayer(%q, %v) succeeded, want error", tt.oldID, tt.player)
				}

				if got := len(table.Players); got != 2 {
					t.Fatalf("len(Players) = %d, want 2", got)
				}
			})
		}
	})
}

func TestSeatPlayer(t *testing.T) {
	t.Run("appends while there is room", func(t *testing.T) {
		table := newLifecycleTable(t, "lifecycle-seat-append", 2, []Chips{100})

		late := NewPlayer("late", stubIDMaker{"late"}, 100)
		if err := late.AddCard(mustCards(t, "AS")[0]); err != nil {
			t.Fatalf("AddCard: %v", err)
		}

		if err := table.SeatPlayer(late); err != nil {
			t.Fatalf("SeatPlayer: %v", err)
		}

		if got := len(table.Players); got != 2 {
			t.Fatalf("len(Players) = %d, want 2", got)
		}

		if got := table.Players[1].ID; got != "late" {
			t.Fatalf("Players[1].ID = %q, want late", got)
		}

		if got := len(late.GetPocketCards()); got != 0 {
			t.Fatalf("late player has %d pocket cards, want 0", got)
		}

		if _, err := table.Pot.GetPlayerAmount("late"); err != nil {
			t.Fatalf("seat player is not registered in the pot: %v", err)
		}
	})

	t.Run("replaces the first busted seat when full", func(t *testing.T) {
		table := newLifecycleTable(t, "lifecycle-seat-replace", 3, []Chips{100, 0, 100})

		late := NewPlayer("late", stubIDMaker{"late"}, 100)
		if err := table.SeatPlayer(late); err != nil {
			t.Fatalf("SeatPlayer: %v", err)
		}

		if got := len(table.Players); got != 3 {
			t.Fatalf("len(Players) = %d, want 3", got)
		}

		if got := table.Players[1].ID; got != "late" {
			t.Fatalf("Players[1].ID = %q, want late", got)
		}

		if table.GetPlayerByID("p1") != nil {
			t.Fatal("busted player is still at the table")
		}
	})

	t.Run("errors when every seat is funded", func(t *testing.T) {
		table := newLifecycleTable(t, "lifecycle-seat-full", 2, []Chips{100, 100})

		if err := table.SeatPlayer(NewPlayer("late", stubIDMaker{"late"}, 100)); err == nil {
			t.Fatal("SeatPlayer succeeded on a full funded table")
		}

		if got := len(table.Players); got != 2 {
			t.Fatalf("len(Players) = %d, want 2", got)
		}

		if table.GetPlayerByID("late") != nil {
			t.Fatal("rejected player was seated")
		}
	})

	t.Run("rejects invalid seats", func(t *testing.T) {
		table := newLifecycleTable(t, "lifecycle-seat-invalid", 2, []Chips{100})

		if err := table.SeatPlayer(nil); err == nil {
			t.Fatal("SeatPlayer(nil) succeeded")
		}

		if err := table.SeatPlayer(NewPlayer("p0", stubIDMaker{"p0"}, 100)); err == nil {
			t.Fatal("SeatPlayer accepted a duplicate ID")
		}

		if err := table.SeatPlayer(NewPlayer("broke", stubIDMaker{"broke"}, 0)); err == nil {
			t.Fatal("SeatPlayer accepted a player without chips")
		}

		if got := len(table.Players); got != 1 {
			t.Fatalf("len(Players) = %d, want 1", got)
		}
	})

	t.Run("rejects mid-hand calls", func(t *testing.T) {
		table := newEngineTable(t, []Chips{100, 100}, 5, 10)

		if err := table.SeatPlayer(NewPlayer("late", stubIDMaker{"late"}, 100)); err == nil {
			t.Fatal("SeatPlayer succeeded during betting")
		}

		if table.GetPlayerByID("late") != nil {
			t.Fatal("mid-hand seat request was applied")
		}
	})
}

func TestTableAddChips(t *testing.T) {
	t.Run("only between hands", func(t *testing.T) {
		table := newEngineTable(t, []Chips{100, 100}, 5, 10)
		before := table.GetPlayerByID("p0").GetCurrentChipsAmount()

		if err := table.AddChips("p0", 50); err == nil {
			t.Fatal("AddChips succeeded during betting")
		}

		if got := table.GetPlayerByID("p0").GetCurrentChipsAmount(); got != before {
			t.Fatalf("p0 chips = %d after rejected AddChips, want %d", got, before)
		}
	})

	t.Run("rejects non-positive amounts", func(t *testing.T) {
		table := newHeadsUpWaitingTable(t, "lifecycle-add-invalid", 100, 5, 10)

		for _, amount := range []Chips{0, -5} {
			if err := table.AddChips("p0", amount); err == nil {
				t.Fatalf("AddChips accepted amount %d", amount)
			}
		}

		if got := table.GetPlayerByID("p0").GetCurrentChipsAmount(); got != 100 {
			t.Fatalf("p0 chips = %d, want 100", got)
		}
	})

	t.Run("rejects unknown players", func(t *testing.T) {
		table := newHeadsUpWaitingTable(t, "lifecycle-add-unknown", 100, 5, 10)

		if err := table.AddChips("missing", 50); err == nil {
			t.Fatal("AddChips accepted an unknown player")
		}
	})

	t.Run("reactivates a funded player", func(t *testing.T) {
		table := newHeadsUpWaitingTable(t, "lifecycle-add-active", 100, 5, 10)
		p := table.GetPlayerByID("p0")
		p.Active = false

		if err := table.AddChips("p0", 50); err != nil {
			t.Fatalf("AddChips: %v", err)
		}

		if got := p.GetCurrentChipsAmount(); got != 150 {
			t.Fatalf("p0 chips = %d, want 150", got)
		}

		if !p.Active {
			t.Fatal("funded player is inactive after AddChips")
		}
	})

	t.Run("keeps a sitting-out player inactive", func(t *testing.T) {
		table := newHeadsUpWaitingTable(t, "lifecycle-add-sitout", 100, 5, 10)

		if err := table.SitOut("p0"); err != nil {
			t.Fatalf("SitOut: %v", err)
		}

		if err := table.AddChips("p0", 50); err != nil {
			t.Fatalf("AddChips: %v", err)
		}

		p := table.GetPlayerByID("p0")

		if got := p.GetCurrentChipsAmount(); got != 150 {
			t.Fatalf("p0 chips = %d, want 150", got)
		}

		if !p.IsSittingOut() {
			t.Fatal("AddChips cleared the sit-out flag")
		}

		if p.Active {
			t.Fatal("sitting-out player became active after AddChips")
		}

		if _, err := table.StartHand(); err == nil {
			t.Fatal("StartHand succeeded with one funded player after a sit-out")
		}
	})
}

func TestTableSitOutAndSitIn(t *testing.T) {
	t.Run("sit-out round trip", func(t *testing.T) {
		table := newHeadsUpWaitingTable(t, "lifecycle-sitout-roundtrip", 100, 5, 10)
		p := table.GetPlayerByID("p0")

		if err := table.SitOut("p0"); err != nil {
			t.Fatalf("SitOut: %v", err)
		}

		if !p.IsSittingOut() || p.Active {
			t.Fatal("SitOut did not deactivate the player")
		}

		if err := table.SitOut("p0"); err != nil {
			t.Fatalf("second SitOut: %v", err)
		}

		if err := table.SitIn("p0"); err != nil {
			t.Fatalf("SitIn: %v", err)
		}

		if p.IsSittingOut() || !p.Active {
			t.Fatal("SitIn did not reactivate the player")
		}
	})

	t.Run("rejects mid-hand calls", func(t *testing.T) {
		table := newEngineTable(t, []Chips{100, 100}, 5, 10)

		if err := table.SitOut("p0"); err == nil {
			t.Fatal("SitOut succeeded during betting")
		}

		if err := table.SitIn("p0"); err == nil {
			t.Fatal("SitIn succeeded during betting")
		}
	})

	t.Run("SitIn rejects a player without chips", func(t *testing.T) {
		table := newLifecycleTable(t, "lifecycle-sitin-broke", 0, []Chips{100, 0})

		if err := table.SitOut("p1"); err != nil {
			t.Fatalf("SitOut: %v", err)
		}

		if err := table.SitIn("p1"); err == nil {
			t.Fatal("SitIn succeeded for a player without chips")
		}

		if !table.GetPlayerByID("p1").IsSittingOut() {
			t.Fatal("rejected SitIn cleared the sit-out flag")
		}
	})

	t.Run("rejects unknown players", func(t *testing.T) {
		table := newHeadsUpWaitingTable(t, "lifecycle-sit-unknown", 100, 5, 10)

		if err := table.SitOut("missing"); err == nil {
			t.Fatal("SitOut accepted an unknown player")
		}

		if err := table.SitIn("missing"); err == nil {
			t.Fatal("SitIn accepted an unknown player")
		}
	})
}

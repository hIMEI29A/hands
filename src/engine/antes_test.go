package engine

import (
	"fmt"
	"testing"
)

// newAnteTable builds a table with deterministic IDs and a fixed dealer at
// seat 0, then starts a hand.
func newAnteTable(t *testing.T, stacks []Chips, sb, bb, ante Chips, mode AnteMode) *Table {
	t.Helper()

	rules := DefaultTableRules(sb, bb)
	rules.Ante = ante
	rules.AnteMode = mode

	table, err := NewTableWithRules("antes", "tag", stubIDMaker{"antes"}, CashTableType, 0, rules)
	if err != nil {
		t.Fatalf("NewTableWithRules: %v", err)
	}

	for i, stack := range stacks {
		id := fmt.Sprintf("p%d", i)
		if err := table.Register(NewPlayer(id, stubIDMaker{id}, stack)); err != nil {
			t.Fatalf("Register(%s): %v", id, err)
		}
	}

	table.isDealerInactive = false
	table.Dealer = -1

	if _, err := table.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	return table
}

func mustContribution(t *testing.T, table *Table, playerID string) Chips {
	t.Helper()

	amount, err := table.Pot.GetPlayerAmount(playerID)
	if err != nil {
		t.Fatalf("GetPlayerAmount(%s): %v", playerID, err)
	}

	return Chips(amount)
}

// checkDownToShowdown checks and calls NextStreet until the hand reaches
// the showdown phase. Every postflop betting round must be checkable, i.e.
// no player is facing a bet.
func checkDownToShowdown(t *testing.T, table *Table) {
	t.Helper()

	for table.Phase() != PhaseShowdown {
		if table.Phase() != PhaseBetting {
			if _, err := table.NextStreet(); err != nil {
				t.Fatalf("NextStreet: %v", err)
			}

			continue
		}

		actor, err := table.CurrentActor()
		if err != nil {
			t.Fatalf("CurrentActor: %v", err)
		}

		mustAct(t, table, actor.ID, ActionCheck, 0)
	}
}

func TestAntesPerPlayerThreeHanded(t *testing.T) {
	table := newAnteTable(t, []Chips{1000, 1000, 1000}, 10, 20, 5, AntePerPlayer)
	initial := totalChips(t, table)

	if got := table.Dealer; got != 0 {
		t.Fatalf("Dealer = %d, want 0", got)
	}

	if got := table.Pot.TotalChipsNum; got != 45 {
		t.Fatalf("pot = %d, want 45", got)
	}

	wantContributions := map[string]Chips{"p0": 5, "p1": 15, "p2": 25}
	for id, want := range wantContributions {
		if got := mustContribution(t, table, id); got != want {
			t.Fatalf("contribution %s = %d, want %d", id, got, want)
		}
	}

	wantStacks := []Chips{995, 985, 975}
	for i, want := range wantStacks {
		if got := table.Players[i].GetCurrentChipsAmount(); got != want {
			t.Fatalf("p%d stack = %d, want %d", i, got, want)
		}
	}

	wantStreetBets := []Chips{0, 10, 20}
	for i, want := range wantStreetBets {
		if got := table.Players[i].GetStreetBet(); got != want {
			t.Fatalf("p%d street bet = %d, want %d", i, got, want)
		}
	}

	if got := table.CurrentBet(); got != 20 {
		t.Fatalf("CurrentBet = %d, want 20", got)
	}

	actor, err := table.CurrentActor()
	if err != nil {
		t.Fatalf("CurrentActor: %v", err)
	}

	if actor.ID != "p0" {
		t.Fatalf("CurrentActor = %s, want p0", actor.ID)
	}

	wantToCall := map[string]Chips{"p0": 20, "p1": 10, "p2": 0}
	for id, want := range wantToCall {
		toCall, err := table.ToCall(id)
		if err != nil {
			t.Fatalf("ToCall(%s): %v", id, err)
		}

		if toCall != want {
			t.Fatalf("ToCall(%s) = %d, want %d", id, toCall, want)
		}
	}

	if got := totalChips(t, table); got != initial {
		t.Fatalf("total chips = %d, want %d", got, initial)
	}
}

func TestAntesHeadsUp(t *testing.T) {
	table := newAnteTable(t, []Chips{1000, 1000}, 1, 2, 1, AntePerPlayer)

	if got := table.Pot.TotalChipsNum; got != 5 {
		t.Fatalf("pot = %d, want 5", got)
	}

	if got := mustContribution(t, table, "p0"); got != 2 {
		t.Fatalf("p0 contribution = %d, want 2 (ante + small blind)", got)
	}

	if got := mustContribution(t, table, "p1"); got != 3 {
		t.Fatalf("p1 contribution = %d, want 3 (ante + big blind)", got)
	}

	actor, err := table.CurrentActor()
	if err != nil {
		t.Fatalf("CurrentActor: %v", err)
	}

	if actor.ID != "p0" {
		t.Fatalf("CurrentActor = %s, want p0", actor.ID)
	}

	toCall, err := table.ToCall("p0")
	if err != nil {
		t.Fatalf("ToCall(p0): %v", err)
	}

	if toCall != 1 {
		t.Fatalf("ToCall(p0) = %d, want 1", toCall)
	}
}

func TestAntesShortAllInAndSidePot(t *testing.T) {
	table := newAnteTable(t, []Chips{1, 100, 100}, 1, 2, 2, AntePerPlayer)
	initial := totalChips(t, table)

	p0 := table.Players[0]
	if !p0.IsAllIn() {
		t.Fatal("p0 is not all-in after a short ante")
	}

	if !p0.Active {
		t.Fatal("p0 is not active after a short ante")
	}

	if got := len(p0.GetPocketCards()); got != PocketSize {
		t.Fatalf("p0 has %d pocket cards, want %d", got, PocketSize)
	}

	if got := mustContribution(t, table, "p0"); got != 1 {
		t.Fatalf("p0 contribution = %d, want 1", got)
	}

	actor, err := table.CurrentActor()
	if err != nil {
		t.Fatalf("CurrentActor: %v", err)
	}

	if actor.ID != "p1" {
		t.Fatalf("CurrentActor = %s, want p1 (p0 is skipped)", actor.ID)
	}

	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if got := table.Pot.TotalChipsNum; got != 9 {
		t.Fatalf("pot = %d, want 9", got)
	}

	checkDownToShowdown(t, table)

	setBoard(t, table, "2H", "3D", "5S", "9H", "KD")
	replacePocket(t, table, "p0", "AS", "AD")
	replacePocket(t, table, "p1", "QS", "QC")
	replacePocket(t, table, "p2", "7C", "8C")

	results, err := table.Showdown()
	if err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2 side pots", len(results))
	}

	main := results[0]
	if main.Amount != 3 || len(main.Winners) != 1 || main.Winners[0] != "p0" {
		t.Fatalf("main pot = %+v, want 3 to p0", main)
	}

	side := results[1]
	if side.Amount != 6 || len(side.Winners) != 1 || side.Winners[0] != "p1" {
		t.Fatalf("side pot = %+v, want 6 to p1", side)
	}

	wantStacks := []Chips{3, 102, 96}
	for i, want := range wantStacks {
		if got := table.Players[i].GetCurrentChipsAmount(); got != want {
			t.Fatalf("p%d final stack = %d, want %d", i, got, want)
		}
	}

	if got := totalChips(t, table); got != initial {
		t.Fatalf("total chips = %d, want %d", got, initial)
	}
}

func TestAntesBigBlindMode(t *testing.T) {
	table := newAnteTable(t, []Chips{1000, 1000, 1000}, 1, 2, 2, AnteBigBlind)

	if got := table.Pot.TotalChipsNum; got != 5 {
		t.Fatalf("pot = %d, want 5", got)
	}

	wantContributions := map[string]Chips{"p0": 0, "p1": 1, "p2": 4}
	for id, want := range wantContributions {
		if got := mustContribution(t, table, id); got != want {
			t.Fatalf("contribution %s = %d, want %d", id, got, want)
		}
	}

	wantStreetBets := []Chips{0, 1, 2}
	for i, want := range wantStreetBets {
		if got := table.Players[i].GetStreetBet(); got != want {
			t.Fatalf("p%d street bet = %d, want %d", i, got, want)
		}
	}

	if got := table.CurrentBet(); got != 2 {
		t.Fatalf("CurrentBet = %d, want 2", got)
	}

	actor, err := table.CurrentActor()
	if err != nil {
		t.Fatalf("CurrentActor: %v", err)
	}

	if actor.ID != "p0" {
		t.Fatalf("CurrentActor = %s, want p0", actor.ID)
	}

	wantToCall := map[string]Chips{"p0": 2, "p1": 1, "p2": 0}
	for id, want := range wantToCall {
		toCall, err := table.ToCall(id)
		if err != nil {
			t.Fatalf("ToCall(%s): %v", id, err)
		}

		if toCall != want {
			t.Fatalf("ToCall(%s) = %d, want %d", id, toCall, want)
		}
	}

	mustAct(t, table, "p0", ActionFold, 0)
	mustAct(t, table, "p1", ActionFold, 0)

	results, err := table.Showdown()
	if err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("results = %d, want refund + pot", len(results))
	}

	if !results[0].Refunded || results[0].Amount != 3 {
		t.Fatalf("refund = %+v, want 3 to p2", results[0])
	}

	if results[1].Amount != 2 || len(results[1].Winners) != 1 || results[1].Winners[0] != "p2" {
		t.Fatalf("pot result = %+v, want 2 to p2", results[1])
	}

	wantStacks := []Chips{1000, 999, 1001}
	for i, want := range wantStacks {
		if got := table.Players[i].GetCurrentChipsAmount(); got != want {
			t.Fatalf("p%d final stack = %d, want %d", i, got, want)
		}
	}
}

func TestAntesBigBlindModeWithShortBigBlind(t *testing.T) {
	table := newAnteTable(t, []Chips{100, 100, 2}, 1, 2, 1, AnteBigBlind)

	p2 := table.Players[2]
	if got := mustContribution(t, table, "p2"); got != 2 {
		t.Fatalf("p2 contribution = %d, want 2 (ante + short big blind)", got)
	}

	if got := table.Pot.TotalChipsNum; got != 3 {
		t.Fatalf("pot = %d, want 3", got)
	}

	if !p2.IsAllIn() {
		t.Fatal("p2 is not all-in after posting a short big blind")
	}

	if got := len(p2.GetPocketCards()); got != PocketSize {
		t.Fatalf("p2 has %d pocket cards, want %d", got, PocketSize)
	}

	if _, err := table.LegalActions("p2"); err == nil {
		t.Fatal("p2 can act while all-in")
	}
}

func TestAntesNotRepostedOnNextStreet(t *testing.T) {
	table := newAnteTable(t, []Chips{1000, 1000, 1000}, 10, 20, 5, AntePerPlayer)

	// StartHand posts the antes and blinds once.
	if got := table.Pot.TotalChipsNum; got != 45 {
		t.Fatalf("pot after StartHand = %d, want 45", got)
	}

	wantContributions := map[string]Chips{"p0": 5, "p1": 15, "p2": 25}
	for id, want := range wantContributions {
		if got := mustContribution(t, table, id); got != want {
			t.Fatalf("contribution %s after StartHand = %d, want %d", id, got, want)
		}
	}

	// Complete preflop betting. The calls increase the pot, but the antes
	// must not be posted a second time.
	mustAct(t, table, "p0", ActionCall, 0)
	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if got := table.Phase(); got != PhaseStreetComplete {
		t.Fatalf("Phase = %q, want %q", got, PhaseStreetComplete)
	}

	potBefore := table.Pot.TotalChipsNum
	if potBefore != 75 {
		t.Fatalf("pot after preflop = %d, want 75", potBefore)
	}

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet: %v", err)
	}

	if got := table.Pot.TotalChipsNum; got != potBefore {
		t.Fatalf("pot after NextStreet = %d, want %d (antes re-posted)", got, potBefore)
	}

	for id, want := range map[string]Chips{"p0": 25, "p1": 25, "p2": 25} {
		if got := mustContribution(t, table, id); got != want {
			t.Fatalf("contribution %s after NextStreet = %d, want %d", id, got, want)
		}
	}
}

func TestAntesFoldedAnteRemainsDeadMoney(t *testing.T) {
	table := newAnteTable(t, []Chips{1000, 1000, 1000}, 10, 20, 5, AntePerPlayer)

	mustAct(t, table, "p0", ActionFold, 0)
	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if got := table.Pot.TotalChipsNum; got != 55 {
		t.Fatalf("pot = %d, want 55", got)
	}

	if got := mustContribution(t, table, "p0"); got != 5 {
		t.Fatalf("folded p0 contribution = %d, want 5 (ante stays dead money)", got)
	}

	checkDownToShowdown(t, table)

	setBoard(t, table, "10H", "JH", "QH", "KH", "AH")
	replacePocket(t, table, "p0", "AS", "AD")
	replacePocket(t, table, "p1", "2C", "3C")
	replacePocket(t, table, "p2", "2D", "3D")

	results, err := table.Showdown()
	if err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	var total Chips
	payouts := map[string]Chips{}

	for _, result := range results {
		total += result.Amount

		for id, amount := range result.Payouts {
			payouts[id] += amount
		}
	}

	if total != 55 {
		t.Fatalf("results total = %d, want 55", total)
	}

	if _, ok := payouts["p0"]; ok {
		t.Fatal("folded p0 received chips")
	}
}

func TestAntesUncalledRefundIncludesAnte(t *testing.T) {
	table := newAnteTable(t, []Chips{1000, 1000}, 1, 2, 1, AntePerPlayer)

	mustAct(t, table, "p0", ActionRaise, 10)
	mustAct(t, table, "p1", ActionFold, 0)

	results, err := table.Showdown()
	if err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("results = %d, want refund + pot", len(results))
	}

	if !results[0].Refunded || results[0].Amount != 8 {
		t.Fatalf("refund = %+v, want 8 to p0", results[0])
	}

	if results[1].Amount != 6 || len(results[1].Winners) != 1 || results[1].Winners[0] != "p0" {
		t.Fatalf("pot result = %+v, want 6 to p0", results[1])
	}

	if got := table.Players[0].GetCurrentChipsAmount(); got != 1003 {
		t.Fatalf("p0 final stack = %d, want 1003", got)
	}

	if got := table.Players[1].GetCurrentChipsAmount(); got != 997 {
		t.Fatalf("p1 final stack = %d, want 997", got)
	}
}

func TestAntesAllPlayersAllInFromAntes(t *testing.T) {
	table := newAnteTable(t, []Chips{1, 1, 1}, 1, 2, 1, AntePerPlayer)

	if got := table.Phase(); got != PhaseStreetComplete {
		t.Fatalf("Phase = %q, want %q", got, PhaseStreetComplete)
	}

	if got := table.Pot.TotalChipsNum; got != 3 {
		t.Fatalf("pot = %d, want 3", got)
	}

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet: %v", err)
	}

	if got := table.Phase(); got != PhaseShowdown {
		t.Fatalf("Phase = %q, want %q", got, PhaseShowdown)
	}

	if got := len(table.Board); got != BoardSize {
		t.Fatalf("board size = %d, want %d", got, BoardSize)
	}

	results, err := table.Showdown()
	if err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	var total Chips
	for _, result := range results {
		total += result.Amount
	}

	if total != 3 {
		t.Fatalf("payout total = %d, want 3", total)
	}
}

func TestBlindsPostsAntes(t *testing.T) {
	rules := DefaultTableRules(1, 2)
	rules.Ante = 2

	table, err := NewTableWithRules("legacy-antes", "tag", stubIDMaker{"legacy-antes"}, CashTableType, 0, rules)
	if err != nil {
		t.Fatalf("NewTableWithRules: %v", err)
	}

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("p%d", i)
		if err := table.Register(NewPlayer(id, stubIDMaker{id}, 1000)); err != nil {
			t.Fatalf("Register(%s): %v", id, err)
		}
	}

	table.Dealer = 0
	table.isDealerInactive = false

	if _, err := table.Blinds(); err != nil {
		t.Fatalf("Blinds: %v", err)
	}

	wantContributions := map[string]Chips{"p0": 2, "p1": 3, "p2": 4}
	for id, want := range wantContributions {
		if got := mustContribution(t, table, id); got != want {
			t.Fatalf("contribution %s = %d, want %d", id, got, want)
		}
	}

	if got := table.Pot.TotalChipsNum; got != 9 {
		t.Fatalf("pot = %d, want 9", got)
	}

	if got := table.CurrentBet(); got != 2 {
		t.Fatalf("CurrentBet = %d, want 2", got)
	}
}

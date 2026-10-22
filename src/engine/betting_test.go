package engine

import (
	"fmt"
	"testing"
)

// newEngineTable builds a table with deterministic IDs and a fixed dealer
// at seat 0, then starts a hand.
func newEngineTable(t *testing.T, stacks []Chips, sb, bb Chips) *Table {
	t.Helper()

	table := newTestTable("engine", "tag", 0, int(bb), int(sb))

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

func mustAct(t *testing.T, table *Table, playerID string, action ActionType, amount Chips) ActionResult {
	t.Helper()

	result, err := table.Act(Action{PlayerID: playerID, Type: action, Amount: amount})
	if err != nil {
		t.Fatalf("Act(%s, %s, %d): %v", playerID, action, amount, err)
	}

	return result
}

func mustNotAct(t *testing.T, table *Table, playerID string, action ActionType, amount Chips) {
	t.Helper()

	if _, err := table.Act(Action{PlayerID: playerID, Type: action, Amount: amount}); err == nil {
		t.Fatalf("Act(%s, %s, %d) succeeded, want error", playerID, action, amount)
	}
}

func replacePocket(t *testing.T, table *Table, playerID string, cards ...string) {
	t.Helper()

	p := table.GetPlayerByID(playerID)
	if p == nil {
		t.Fatalf("player %q not found", playerID)
	}

	p.mu.Lock()
	p.pocketCards = mustCards(t, cards...)
	p.mu.Unlock()
}

func hasAction(actions []ActionType, want ActionType) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}

	return false
}

func totalChips(t *testing.T, table *Table) Chips {
	t.Helper()

	table.m.RLock()
	defer table.m.RUnlock()

	total := table.Pot.TotalChipsNum

	for _, p := range table.Players {
		total += p.GetCurrentChipsAmount()
	}

	return total
}

func TestPreflopOrderAndPositions(t *testing.T) {
	t.Run("three-handed", func(t *testing.T) {
		table := newEngineTable(t, []Chips{1000, 1000, 1000}, 10, 20)

		if got := table.Dealer; got != 0 {
			t.Fatalf("Dealer = %d, want 0", got)
		}

		actor, err := table.CurrentActor()
		if err != nil {
			t.Fatalf("CurrentActor: %v", err)
		}

		if actor.ID != "p0" {
			t.Fatalf("CurrentActor = %s, want p0", actor.ID)
		}
	})

	t.Run("heads-up dealer acts first preflop", func(t *testing.T) {
		table := newEngineTable(t, []Chips{1000, 1000}, 10, 20)

		dealer, err := table.GetDealer()
		if err != nil {
			t.Fatalf("GetDealer: %v", err)
		}

		actor, err := table.CurrentActor()
		if err != nil {
			t.Fatalf("CurrentActor: %v", err)
		}

		if actor.ID != dealer.ID {
			t.Fatalf("CurrentActor = %s, want dealer %s", actor.ID, dealer.ID)
		}

		// p0 is the dealer/small blind: 10 posted, 20 to match.
		toCall, err := table.ToCall("p0")
		if err != nil {
			t.Fatalf("ToCall(p0): %v", err)
		}

		if toCall != 10 {
			t.Fatalf("ToCall(p0) = %d, want 10", toCall)
		}
	})
}

func TestBettingRoundChecksAndCalls(t *testing.T) {
	table := newEngineTable(t, []Chips{1000, 1000, 1000}, 10, 20)
	initial := totalChips(t, table)

	result := mustAct(t, table, "p0", ActionCall, 0)
	if result.Paid != 20 {
		t.Fatalf("p0 call paid %d, want 20", result.Paid)
	}

	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if got := table.Phase(); got != PhaseStreetComplete {
		t.Fatalf("Phase = %q, want %q", got, PhaseStreetComplete)
	}

	if got := table.Pot.TotalChipsNum; got != 60 {
		t.Fatalf("pot = %d, want 60", got)
	}

	if got := totalChips(t, table); got != initial {
		t.Fatalf("total chips = %d, want %d", got, initial)
	}

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet: %v", err)
	}

	if got := table.Street(); got != StreetFlop {
		t.Fatalf("Street = %q, want %q", got, StreetFlop)
	}

	if got := len(table.Board); got != CardsOnFlopNumber {
		t.Fatalf("board size = %d, want %d", got, CardsOnFlopNumber)
	}

	actor, err := table.CurrentActor()
	if err != nil {
		t.Fatalf("CurrentActor: %v", err)
	}

	// Postflop action starts left of the dealer (the small blind).
	if actor.ID != "p1" {
		t.Fatalf("flop actor = %s, want p1", actor.ID)
	}

	mustAct(t, table, "p1", ActionCheck, 0)
	mustAct(t, table, "p2", ActionCheck, 0)
	mustAct(t, table, "p0", ActionCheck, 0)

	if got := table.Phase(); got != PhaseStreetComplete {
		t.Fatalf("Phase after flop = %q, want %q", got, PhaseStreetComplete)
	}

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet: %v", err)
	}

	if got := len(table.Board); got != 4 {
		t.Fatalf("board size after turn = %d, want 4", got)
	}

	if got := totalChips(t, table); got != initial {
		t.Fatalf("total chips = %d, want %d", got, initial)
	}
}

func TestBettingActionValidation(t *testing.T) {
	table := newEngineTable(t, []Chips{1000, 1000, 1000}, 10, 20)

	mustNotAct(t, table, "p1", ActionCall, 0)   // out of turn
	mustNotAct(t, table, "p0", ActionCheck, 0)  // facing the big blind
	mustNotAct(t, table, "p0", ActionBet, 40)   // a bet already exists
	mustNotAct(t, table, "p0", ActionRaise, 30) // below minimum raise
	mustNotAct(t, table, "p0", ActionType("x"), 0)

	mustAct(t, table, "p0", ActionCall, 0)

	actions, err := table.LegalActions("p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}

	if !hasAction(actions, ActionCall) || !hasAction(actions, ActionRaise) || !hasAction(actions, ActionFold) {
		t.Fatalf("p1 actions = %v, want call/raise/fold", actions)
	}

	mustNotAct(t, table, "p1", ActionRaise, 30)

	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if got := table.Phase(); got != PhaseStreetComplete {
		t.Fatalf("Phase = %q, want %q", got, PhaseStreetComplete)
	}
}

func TestBigBlindOptionCanRaise(t *testing.T) {
	table := newEngineTable(t, []Chips{1000, 1000, 1000}, 10, 20)

	mustAct(t, table, "p0", ActionCall, 0)
	mustAct(t, table, "p1", ActionCall, 0)

	actions, err := table.LegalActions("p2")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}

	if !hasAction(actions, ActionCheck) || !hasAction(actions, ActionRaise) {
		t.Fatalf("big blind actions = %v, want check and raise", actions)
	}

	mustAct(t, table, "p2", ActionRaise, 40)

	if got := table.CurrentBet(); got != 40 {
		t.Fatalf("CurrentBet = %d, want 40", got)
	}
}

func TestShortAllInRaiseDoesNotReopen(t *testing.T) {
	table := newEngineTable(t, []Chips{150, 1000, 1000}, 10, 20)

	// Preflop: everyone sees a flop with 60 in the pot.
	mustAct(t, table, "p0", ActionCall, 0)
	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet: %v", err)
	}

	// Flop: p1 bets 100, p2 calls, p0 raises all-in to 130.
	mustAct(t, table, "p1", ActionBet, 100)
	mustAct(t, table, "p2", ActionCall, 0)

	result := mustAct(t, table, "p0", ActionRaise, 130)
	if result.Paid != 130 {
		t.Fatalf("p0 paid %d, want 130", result.Paid)
	}

	if got := table.CurrentBet(); got != 130 {
		t.Fatalf("CurrentBet = %d, want 130", got)
	}

	actions, err := table.LegalActions("p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}

	if hasAction(actions, ActionRaise) {
		t.Fatalf("p1 actions = %v, raise must not be available after a short all-in", actions)
	}

	mustNotAct(t, table, "p1", ActionRaise, 300)

	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCall, 0)

	if got := table.Phase(); got != PhaseStreetComplete {
		t.Fatalf("Phase = %q, want %q", got, PhaseStreetComplete)
	}

	if got := table.Pot.TotalChipsNum; got != 450 {
		t.Fatalf("pot = %d, want 450", got)
	}

	if got := table.Players[0].GetCurrentChipsAmount(); got != 0 {
		t.Fatalf("p0 stack = %d, want 0", got)
	}

	if got := table.Players[1].GetCurrentChipsAmount(); got != 850 {
		t.Fatalf("p1 stack = %d, want 850", got)
	}

	if got := table.Players[2].GetCurrentChipsAmount(); got != 850 {
		t.Fatalf("p2 stack = %d, want 850", got)
	}
}

func TestFullRaiseReopensAction(t *testing.T) {
	table := newEngineTable(t, []Chips{1000, 1000, 1000}, 10, 20)

	mustAct(t, table, "p0", ActionRaise, 40)
	mustAct(t, table, "p1", ActionRaise, 60)
	mustAct(t, table, "p2", ActionCall, 0)

	actions, err := table.LegalActions("p0")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}

	if !hasAction(actions, ActionRaise) {
		t.Fatalf("p0 actions = %v, want raise after a full raise", actions)
	}

	mustAct(t, table, "p0", ActionRaise, 100)
	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCall, 0)

	if got := table.Pot.TotalChipsNum; got != 300 {
		t.Fatalf("pot = %d, want 300", got)
	}
}

func TestMaxRaisesPerStreet(t *testing.T) {
	rules := DefaultTableRules(10, 20)
	rules.MaxRaisesPerStreet = 1

	table, err := NewTableWithRules("limited", "tag", stubIDMaker{"limited"}, CashTableType, 0, rules)
	if err != nil {
		t.Fatalf("NewTableWithRules: %v", err)
	}

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("p%d", i)
		if err := table.Register(NewPlayer(id, stubIDMaker{id}, 1000)); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	table.isDealerInactive = false
	table.Dealer = -1

	if _, err := table.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	mustAct(t, table, "p0", ActionRaise, 40)
	mustNotAct(t, table, "p1", ActionRaise, 60)

	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCall, 0)

	if got := table.Phase(); got != PhaseStreetComplete {
		t.Fatalf("Phase = %q, want %q", got, PhaseStreetComplete)
	}
}

func TestShortAllInCall(t *testing.T) {
	table := newEngineTable(t, []Chips{15, 1000, 1000}, 10, 20)

	result := mustAct(t, table, "p0", ActionCall, 0)
	if result.Paid != 15 {
		t.Fatalf("p0 paid %d, want 15", result.Paid)
	}

	if !table.Players[0].IsAllIn() {
		t.Fatal("p0 is not all-in after a short call")
	}

	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if got := table.Pot.TotalChipsNum; got != 55 {
		t.Fatalf("pot = %d, want 55", got)
	}

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet: %v", err)
	}

	if got := len(table.Board); got != CardsOnFlopNumber {
		t.Fatalf("board size = %d, want %d", got, CardsOnFlopNumber)
	}
}

func TestAllInRunoutAndShowdown(t *testing.T) {
	table := newEngineTable(t, []Chips{20, 20}, 10, 20)
	initial := totalChips(t, table)

	mustAct(t, table, "p0", ActionCall, 0)

	if got := table.Phase(); got != PhaseStreetComplete {
		t.Fatalf("Phase = %q, want %q", got, PhaseStreetComplete)
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

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	if got := table.Phase(); got != PhaseComplete {
		t.Fatalf("Phase = %q, want %q", got, PhaseComplete)
	}

	if got := table.Pot.TotalChipsNum; got != 0 {
		t.Fatalf("pot = %d, want 0", got)
	}

	if got := totalChips(t, table); got != initial {
		t.Fatalf("total chips = %d, want %d", got, initial)
	}
}

func TestUncalledBetRefund(t *testing.T) {
	table := newEngineTable(t, []Chips{1000, 1000}, 10, 20)

	mustAct(t, table, "p0", ActionRaise, 100)
	mustAct(t, table, "p1", ActionFold, 0)

	if got := table.Phase(); got != PhaseShowdown {
		t.Fatalf("Phase = %q, want %q", got, PhaseShowdown)
	}

	results, err := table.Showdown()
	if err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("results = %d, want refund + pot", len(results))
	}

	if !results[0].Refunded || results[0].Amount != 80 {
		t.Fatalf("refund = %+v, want 80 to p0", results[0])
	}

	if results[1].Amount != 40 || results[1].Winners[0] != "p0" {
		t.Fatalf("pot result = %+v, want 40 to p0", results[1])
	}

	if got := table.Players[0].GetCurrentChipsAmount(); got != 1020 {
		t.Fatalf("p0 stack = %d, want 1020", got)
	}

	if got := table.Players[1].GetCurrentChipsAmount(); got != 980 {
		t.Fatalf("p1 stack = %d, want 980", got)
	}

	if got := table.Pot.TotalChipsNum; got != 0 {
		t.Fatalf("pot = %d, want 0", got)
	}
}

func TestShowdownSidePots(t *testing.T) {
	table := newEngineTable(t, []Chips{50, 300, 300}, 10, 20)
	initial := totalChips(t, table)

	mustAct(t, table, "p0", ActionRaise, 50)
	mustAct(t, table, "p1", ActionRaise, 150)
	mustAct(t, table, "p2", ActionCall, 0)

	if got := table.Pot.TotalChipsNum; got != 350 {
		t.Fatalf("pot = %d, want 350", got)
	}

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet flop: %v", err)
	}

	mustAct(t, table, "p1", ActionCheck, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet turn: %v", err)
	}

	mustAct(t, table, "p1", ActionCheck, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet river: %v", err)
	}

	mustAct(t, table, "p1", ActionCheck, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet showdown: %v", err)
	}

	setBoard(t, table, "2H", "3D", "5S", "9H", "KD")
	replacePocket(t, table, "p0", "9S", "9D")
	replacePocket(t, table, "p1", "AS", "AD")
	replacePocket(t, table, "p2", "2C", "3C")

	results, err := table.Showdown()
	if err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2 side pots", len(results))
	}

	main := results[0]
	if main.Amount != 150 || main.Winners[0] != "p0" {
		t.Fatalf("main pot = %+v, want 150 to p0", main)
	}

	side := results[1]
	if side.Amount != 200 || side.Winners[0] != "p2" {
		t.Fatalf("side pot = %+v, want 200 to p2", side)
	}

	if got := table.Players[0].GetCurrentChipsAmount(); got != 150 {
		t.Fatalf("p0 stack = %d, want 150", got)
	}

	if got := table.Players[1].GetCurrentChipsAmount(); got != 150 {
		t.Fatalf("p1 stack = %d, want 150", got)
	}

	if got := table.Players[2].GetCurrentChipsAmount(); got != 350 {
		t.Fatalf("p2 stack = %d, want 350", got)
	}

	if got := totalChips(t, table); got != initial {
		t.Fatalf("total chips = %d, want %d", got, initial)
	}

	if got := table.Pot.TotalChipsNum; got != 0 {
		t.Fatalf("pot = %d, want 0", got)
	}

	last := table.LastResults()
	if len(last) != 2 {
		t.Fatalf("LastResults = %d, want 2", len(last))
	}
}

func TestShowdownSplitPot(t *testing.T) {
	table := newEngineTable(t, []Chips{1000, 1000, 1000}, 10, 20)

	mustAct(t, table, "p0", ActionCall, 0)
	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet flop: %v", err)
	}

	mustAct(t, table, "p1", ActionCheck, 0)
	mustAct(t, table, "p2", ActionCheck, 0)
	mustAct(t, table, "p0", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet turn: %v", err)
	}

	mustAct(t, table, "p1", ActionCheck, 0)
	mustAct(t, table, "p2", ActionCheck, 0)
	mustAct(t, table, "p0", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet river: %v", err)
	}

	mustAct(t, table, "p1", ActionCheck, 0)
	mustAct(t, table, "p2", ActionCheck, 0)
	mustAct(t, table, "p0", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet showdown: %v", err)
	}

	// A royal flush on the board makes every hand tie.
	setBoard(t, table, "10H", "JH", "QH", "KH", "AH")
	replacePocket(t, table, "p0", "2C", "3C")
	replacePocket(t, table, "p1", "2D", "3D")
	replacePocket(t, table, "p2", "2S", "3S")

	results, err := table.Showdown()
	if err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if len(results) != 1 || results[0].Amount != 60 {
		t.Fatalf("results = %+v, want one 60-chip pot", results)
	}

	for _, id := range []string{"p0", "p1", "p2"} {
		if got := results[0].Payouts[id]; got != 20 {
			t.Fatalf("payout to %s = %d, want 20", id, got)
		}
	}
}

func TestSplitPayoutsOddChip(t *testing.T) {
	order := map[string]int{"p0": 0, "p1": 1, "p2": 2}

	payouts := splitPayouts(25, []string{"p1", "p0"}, order)

	if payouts["p0"] != 13 || payouts["p1"] != 12 {
		t.Fatalf("payouts = %v, want p0=13 p1=12 (left of dealer first)", payouts)
	}
}

func TestFoldedPlayerContributesButCannotWin(t *testing.T) {
	table := newEngineTable(t, []Chips{1000, 1000, 1000}, 10, 20)

	mustAct(t, table, "p0", ActionCall, 0)
	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet flop: %v", err)
	}

	mustAct(t, table, "p1", ActionBet, 20)
	mustAct(t, table, "p2", ActionCall, 0)
	mustAct(t, table, "p0", ActionFold, 0)

	if got := table.Pot.TotalChipsNum; got != 100 {
		t.Fatalf("pot = %d, want 100", got)
	}

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet turn: %v", err)
	}

	mustAct(t, table, "p1", ActionCheck, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet river: %v", err)
	}

	mustAct(t, table, "p1", ActionCheck, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet showdown: %v", err)
	}

	setBoard(t, table, "10H", "JH", "QH", "KH", "AH")
	replacePocket(t, table, "p0", "2C", "3C")
	replacePocket(t, table, "p1", "2D", "3D")
	replacePocket(t, table, "p2", "2S", "3S")

	potBefore := table.Pot.TotalChipsNum
	stacksBefore := []Chips{
		table.Players[0].GetCurrentChipsAmount(),
		table.Players[1].GetCurrentChipsAmount(),
		table.Players[2].GetCurrentChipsAmount(),
	}

	preview, err := table.ResolvePots()
	if err != nil {
		t.Fatalf("ResolvePots: %v", err)
	}

	var previewTotal Chips
	for _, result := range preview {
		previewTotal += result.Amount
	}

	if previewTotal != 100 {
		t.Fatalf("preview total = %d, want 100", previewTotal)
	}

	if table.Pot.TotalChipsNum != potBefore {
		t.Fatal("ResolvePots changed the pot")
	}

	for i, stack := range stacksBefore {
		if got := table.Players[i].GetCurrentChipsAmount(); got != stack {
			t.Fatalf("ResolvePots changed player %d stack: %d -> %d", i, stack, got)
		}
	}

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

	if total != 100 {
		t.Fatalf("results total = %d, want 100", total)
	}

	if payouts["p1"] != 50 || payouts["p2"] != 50 {
		t.Fatalf("payouts = %v, want 50 each for p1 and p2", payouts)
	}

	if _, ok := payouts["p0"]; ok {
		t.Fatal("folded player received chips")
	}

	after, err := table.ResolvePots()
	if err != nil {
		t.Fatalf("ResolvePots after showdown: %v", err)
	}

	if len(after) != len(results) {
		t.Fatalf("ResolvePots after showdown returned %d results, want %d", len(after), len(results))
	}
}

func TestShowdownBeforeBoardCompleteErrors(t *testing.T) {
	table := newEngineTable(t, []Chips{1000, 1000, 1000}, 10, 20)

	mustAct(t, table, "p0", ActionCall, 0)
	mustAct(t, table, "p1", ActionCall, 0)
	mustAct(t, table, "p2", ActionCheck, 0)

	if _, err := table.NextStreet(); err != nil {
		t.Fatalf("NextStreet: %v", err)
	}

	// Pretend the river betting is complete without dealing the remaining
	// board; Showdown must reject the incomplete board.
	table.m.Lock()
	table.phase = PhaseShowdown
	table.m.Unlock()

	if _, err := table.Showdown(); err == nil {
		t.Fatal("Showdown with an incomplete board succeeded")
	}
}

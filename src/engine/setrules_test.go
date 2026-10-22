package engine

import (
	"fmt"
	"testing"
)

// newHeadsUpWaitingTable builds a table with two funded players and no
// hand in progress, so the dealer position is not yet relevant.
func newHeadsUpWaitingTable(t *testing.T, name string, stack, sb, bb Chips) *Table {
	t.Helper()

	table := newTestTable(name, "tag", 0, int(bb), int(sb))

	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("p%d", i)
		if err := table.Register(NewPlayer(id, stubIDMaker{id}, stack)); err != nil {
			t.Fatalf("Register(%s): %v", id, err)
		}
	}

	return table
}

// playHandToShowdown checks or calls through every street until the table
// reaches PhaseShowdown.
func playHandToShowdown(t *testing.T, table *Table) {
	t.Helper()

	for table.Phase() != PhaseShowdown {
		switch table.Phase() {
		case PhaseBetting:
			actor, err := table.CurrentActor()
			if err != nil {
				t.Fatalf("CurrentActor: %v", err)
			}

			toCall, err := table.ToCall(actor.ID)
			if err != nil {
				t.Fatalf("ToCall(%s): %v", actor.ID, err)
			}

			actionType := ActionCheck
			if toCall > 0 {
				actionType = ActionCall
			}

			mustAct(t, table, actor.ID, actionType, 0)

		case PhaseStreetComplete:
			if _, err := table.NextStreet(); err != nil {
				t.Fatalf("NextStreet: %v", err)
			}

		default:
			t.Fatalf("unexpected phase %s while playing a hand", table.Phase())
		}
	}
}

// assertBlindsAndAnte checks the values the next hand must use.
func assertBlindsAndAnte(t *testing.T, table *Table, sb, bb, ante Chips) {
	t.Helper()

	if got := table.SmallBlind(); got != sb {
		t.Fatalf("SmallBlind() = %d, want %d", got, sb)
	}

	if got := table.BigBlind(); got != bb {
		t.Fatalf("BigBlind() = %d, want %d", got, bb)
	}

	if got := table.Ante(); got != ante {
		t.Fatalf("Ante() = %d, want %d", got, ante)
	}
}

func TestSetRulesInWaitingAppliesToNextHand(t *testing.T) {
	table := newHeadsUpWaitingTable(t, "setrules-waiting", 100, 5, 10)

	rules := DefaultTableRules(15, 30)
	rules.Ante = 5

	if err := table.SetRules(rules); err != nil {
		t.Fatalf("SetRules: %v", err)
	}

	assertBlindsAndAnte(t, table, 15, 30, 5)

	if _, err := table.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	assertBlindsAndAnte(t, table, 15, 30, 5)

	// Heads-up with a per-player ante: both players post 5, the dealer
	// posts the small blind and the other player the big blind.
	wantPot := 2*Chips(5) + 15 + 30
	if got := table.Pot.TotalChipsNum; got != wantPot {
		t.Fatalf("pot total = %d, want %d", got, wantPot)
	}
}

func TestSetRulesRejectedDuringBetting(t *testing.T) {
	table := newEngineTable(t, []Chips{100, 100}, 5, 10)

	if table.Phase() != PhaseBetting {
		t.Fatalf("phase = %s, want %s", table.Phase(), PhaseBetting)
	}

	before := table.Rules()

	if err := table.SetRules(DefaultTableRules(50, 100)); err == nil {
		t.Fatal("SetRules succeeded in PhaseBetting, want error")
	}

	if got := table.Rules(); got != before {
		t.Fatalf("rules changed after rejected SetRules: %+v, want %+v", got, before)
	}

	assertBlindsAndAnte(t, table, 5, 10, 0)
}

func TestSetRulesAfterCompleteHandAppliesToNextHand(t *testing.T) {
	table := newHeadsUpWaitingTable(t, "setrules-complete", 100, 5, 10)

	if _, err := table.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	playHandToShowdown(t, table)

	if _, err := table.Showdown(); err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if table.Phase() != PhaseComplete {
		t.Fatalf("phase = %s, want %s", table.Phase(), PhaseComplete)
	}

	rules := DefaultTableRules(20, 40)
	rules.Ante = 10

	if err := table.SetRules(rules); err != nil {
		t.Fatalf("SetRules: %v", err)
	}

	assertBlindsAndAnte(t, table, 20, 40, 10)

	if _, err := table.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	assertBlindsAndAnte(t, table, 20, 40, 10)

	wantPot := 2*Chips(10) + 20 + 40
	if got := table.Pot.TotalChipsNum; got != wantPot {
		t.Fatalf("pot total = %d, want %d", got, wantPot)
	}
}

func TestSetRulesRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TableRules)
	}{
		{"negative ante", func(r *TableRules) { r.Ante = -1 }},
		{"unknown ante mode", func(r *TableRules) { r.AnteMode = "random" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := newHeadsUpWaitingTable(t, "setrules-invalid", 100, 5, 10)

			before := table.Rules()

			rules := DefaultTableRules(15, 30)
			tt.mutate(rules)

			if err := table.SetRules(rules); err == nil {
				t.Fatalf("SetRules accepted invalid rules: %+v", rules)
			}

			if got := table.Rules(); got != before {
				t.Fatalf("rules changed after invalid SetRules: %+v, want %+v", got, before)
			}

			assertBlindsAndAnte(t, table, 5, 10, 0)
		})
	}
}

func TestSetRulesNilReturnsError(t *testing.T) {
	table := newHeadsUpWaitingTable(t, "setrules-nil", 100, 5, 10)

	before := table.Rules()

	if err := table.SetRules(nil); err == nil {
		t.Fatal("SetRules(nil) succeeded, want error")
	}

	if got := table.Rules(); got != before {
		t.Fatalf("rules changed after SetRules(nil): %+v, want %+v", got, before)
	}
}

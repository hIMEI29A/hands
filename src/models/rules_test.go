package models

import "testing"

func TestDefaultTableRules(t *testing.T) {
	rules := DefaultTableRules(10, 20)

	if rules.MinPlayers != 2 {
		t.Fatalf("MinPlayers = %d, want 2", rules.MinPlayers)
	}

	if rules.MinRaise != 20 {
		t.Fatalf("MinRaise = %d, want 20", rules.MinRaise)
	}

	if rules.MaxRaisesPerStreet != 0 {
		t.Fatalf("MaxRaisesPerStreet = %d, want 0 (unlimited)", rules.MaxRaisesPerStreet)
	}

	if rules.Ante != 0 {
		t.Fatalf("Ante = %d, want 0", rules.Ante)
	}

	if rules.AnteMode != AntePerPlayer {
		t.Fatalf("AnteMode = %q, want %q", rules.AnteMode, AntePerPlayer)
	}

	if rules.OddChipRule != OddChipLeftOfDealer {
		t.Fatalf("OddChipRule = %q, want %q", rules.OddChipRule, OddChipLeftOfDealer)
	}

	if err := rules.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestTableRulesValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TableRules)
	}{
		{"min players", func(r *TableRules) { r.MinPlayers = 1 }},
		{"small blind", func(r *TableRules) { r.SmallBlind = 0 }},
		{"big blind", func(r *TableRules) { r.BigBlind = 0 }},
		{"blinds order", func(r *TableRules) { r.BigBlind = 5 }},
		{"min raise", func(r *TableRules) { r.MinRaise = 5 }},
		{"negative ante", func(r *TableRules) { r.Ante = -1 }},
		{"ante mode", func(r *TableRules) { r.AnteMode = "random" }},
		{"negative raise cap", func(r *TableRules) { r.MaxRaisesPerStreet = -1 }},
		{"odd chip rule", func(r *TableRules) { r.OddChipRule = "random" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := DefaultTableRules(10, 20)
			tt.mutate(rules)

			if err := rules.Validate(); err == nil {
				t.Fatalf("Validate accepted invalid rules: %+v", rules)
			}
		})
	}
}

func TestNewTableWithRulesRejectsInvalidRules(t *testing.T) {
	rules := DefaultTableRules(10, 20)
	rules.MinPlayers = 1

	if _, err := NewTableWithRules("bad", "tag", stubIDMaker{"bad"}, CashTableType, 0, rules); err == nil {
		t.Fatal("NewTableWithRules accepted invalid rules")
	}
}

func TestNewTableWithRulesNormalizesOptionalFields(t *testing.T) {
	rules := &TableRules{
		SmallBlind: 10,
		BigBlind:   20,
		Ante:       5,
	}

	table, err := NewTableWithRules("normalized", "tag", stubIDMaker{"normalized"}, CashTableType, 0, rules)
	if err != nil {
		t.Fatalf("NewTableWithRules: %v", err)
	}

	got := table.Rules()
	if got.MinPlayers != 2 || got.MinRaise != 20 || got.OddChipRule != OddChipLeftOfDealer {
		t.Fatalf("normalized rules = %+v", got)
	}

	if got.AnteMode != AntePerPlayer {
		t.Fatalf("normalized AnteMode = %q, want %q", got.AnteMode, AntePerPlayer)
	}

	if got.Ante != 5 {
		t.Fatalf("normalized Ante = %d, want 5", got.Ante)
	}
}

func TestNewTableWithRulesAcceptsBigBlindAnte(t *testing.T) {
	rules := DefaultTableRules(10, 20)
	rules.Ante = 2
	rules.AnteMode = AnteBigBlind

	if err := rules.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	table, err := NewTableWithRules("bba", "tag", stubIDMaker{"bba"}, CashTableType, 0, rules)
	if err != nil {
		t.Fatalf("NewTableWithRules: %v", err)
	}

	if got := table.Ante(); got != 2 {
		t.Fatalf("Ante() = %d, want 2", got)
	}

	if got := table.Rules().AnteMode; got != AnteBigBlind {
		t.Fatalf("AnteMode = %q, want %q", got, AnteBigBlind)
	}
}

package engine

import "fmt"

// OddChipRule controls how an indivisible remainder of a split pot is
// distributed.
type OddChipRule string

const (
	// OddChipLeftOfDealer gives the extra chip(s) to the first winner
	// clockwise from the dealer.
	OddChipLeftOfDealer OddChipRule = "left-of-dealer"
)

// AnteMode controls who posts the ante.
type AnteMode string

const (
	// AntePerPlayer makes every player in the hand post the ante.
	AntePerPlayer AnteMode = "per-player"

	// AnteBigBlind makes only the big blind post the ante.
	AnteBigBlind AnteMode = "big-blind"
)

// TableRules describes the betting rules of a no-limit Texas Hold'em
// table, including antes. A zero value is not valid; use
// DefaultTableRules or NewTableWithRules, which normalize optional
// fields.
type TableRules struct {
	MinPlayers         int
	SmallBlind         Chips
	BigBlind           Chips
	Ante               Chips
	AnteMode           AnteMode
	MinRaise           Chips
	MaxRaisesPerStreet int
	OddChipRule        OddChipRule
}

// DefaultTableRules builds rules for the given blinds. MinRaise defaults to
// the big blind, raises are unlimited, no ante is posted by default and
// odd chips go left of the dealer.
func DefaultTableRules(sb, bb Chips) *TableRules {
	return &TableRules{
		MinPlayers:         2,
		SmallBlind:         sb,
		BigBlind:           bb,
		Ante:               0,
		AnteMode:           AntePerPlayer,
		MinRaise:           bb,
		MaxRaisesPerStreet: 0,
		OddChipRule:        OddChipLeftOfDealer,
	}
}

// normalized returns a copy with optional fields filled in. It does not
// validate the result.
func (r *TableRules) normalized() *TableRules {
	if r == nil {
		return DefaultTableRules(0, 1)
	}

	cp := *r

	if cp.MinPlayers == 0 {
		cp.MinPlayers = 2
	}

	if cp.MinRaise == 0 {
		cp.MinRaise = cp.BigBlind
	}

	if cp.AnteMode == "" {
		cp.AnteMode = AntePerPlayer
	}

	if cp.OddChipRule == "" {
		cp.OddChipRule = OddChipLeftOfDealer
	}

	return &cp
}

// Validate checks the rules for consistency.
func (r *TableRules) Validate() error {
	if r == nil {
		return fmt.Errorf("table rules are nil")
	}

	if r.MinPlayers < 2 {
		return fmt.Errorf("min players must be at least 2, got %d", r.MinPlayers)
	}

	if r.SmallBlind <= 0 {
		return fmt.Errorf("small blind must be positive, got %d", r.SmallBlind)
	}

	if r.BigBlind <= 0 {
		return fmt.Errorf("big blind must be positive, got %d", r.BigBlind)
	}

	if r.BigBlind < r.SmallBlind {
		return fmt.Errorf("big blind %d must not be smaller than small blind %d", r.BigBlind, r.SmallBlind)
	}

	if r.Ante < 0 {
		return fmt.Errorf("ante must not be negative, got %d", r.Ante)
	}

	switch r.AnteMode {
	case "", AntePerPlayer, AnteBigBlind:
	default:
		return fmt.Errorf("unknown ante mode %q", r.AnteMode)
	}

	if r.MinRaise <= 0 {
		return fmt.Errorf("min raise must be positive, got %d", r.MinRaise)
	}

	if r.MinRaise < r.BigBlind {
		return fmt.Errorf("min raise %d must not be smaller than big blind %d", r.MinRaise, r.BigBlind)
	}

	if r.MaxRaisesPerStreet < 0 {
		return fmt.Errorf("max raises per street must not be negative, got %d", r.MaxRaisesPerStreet)
	}

	switch r.OddChipRule {
	case OddChipLeftOfDealer:
	default:
		return fmt.Errorf("unknown odd chip rule %q", r.OddChipRule)
	}

	return nil
}

// SetRules replaces the table rules. It is allowed only between hands,
// in PhaseWaiting or PhaseComplete. The rules are normalized and validated
// before they are applied; the current hand and pot are untouched.
func (t *Table) SetRules(rules *TableRules) error {
	if rules == nil {
		return fmt.Errorf("set rules: table rules are nil")
	}

	normalized := rules.normalized()
	if err := normalized.Validate(); err != nil {
		return fmt.Errorf("invalid table rules: %w", err)
	}

	t.m.Lock()
	defer t.m.Unlock()

	if t.phase != PhaseWaiting && t.phase != PhaseComplete {
		return fmt.Errorf("cannot set rules in phase %s", t.phase)
	}

	t.rules = *normalized

	return nil
}

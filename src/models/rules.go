package models

import "fmt"

// OddChipRule controls how an indivisible remainder of a split pot is
// distributed.
type OddChipRule string

const (
	// OddChipLeftOfDealer gives the extra chip(s) to the first winner
	// clockwise from the dealer.
	OddChipLeftOfDealer OddChipRule = "left-of-dealer"
)

// TableRules describes the betting rules of a no-limit Texas Hold'em
// table. A zero value is not valid; use DefaultTableRules or
// NewTableWithRules, which normalize optional fields.
type TableRules struct {
	MinPlayers         int
	SmallBlind         Chips
	BigBlind           Chips
	MinRaise           Chips
	MaxRaisesPerStreet int
	OddChipRule        OddChipRule
}

// DefaultTableRules builds rules for the given blinds. MinRaise defaults to
// the big blind, raises are unlimited and odd chips go left of the dealer.
func DefaultTableRules(sb, bb Chips) *TableRules {
	return &TableRules{
		MinPlayers:         2,
		SmallBlind:         sb,
		BigBlind:           bb,
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

package models

// Bet is the legacy call/raise amount holder used by the old Player
// helpers.
//
// Deprecated: use Action and Table.Act.
type Bet struct {
	Bet  Chips
	Over Chips
}

// NewBet creates a legacy Bet.
//
// Deprecated: use Action and Table.Act.
func NewBet(bet, over Chips) *Bet {
	return &Bet{
		Bet:  bet,
		Over: over,
	}
}

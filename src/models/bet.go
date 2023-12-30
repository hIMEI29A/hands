package models

type Bet struct {
	Bet  Chips
	Over Chips
}

func NewBet(bet, over Chips) *Bet {
	return &Bet{
		Bet:  bet,
		Over: over,
	}
}

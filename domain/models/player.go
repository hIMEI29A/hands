package models

import "fmt"

type Player struct {
	ID                 string
	Name               string
	PocketCards        []*Card
	CurrentChipsAmount Chips
	Active             bool
}

func (p *Player) Check(bet Chips) (Chips, error) {
	return 0, nil
}

func (p *Player) Call(bet Chips) (Chips, error) {
	if bet > p.CurrentChipsAmount {
		return 0, fmt.Errorf("Wrong bet amount %d", bet)
	}

	p.CurrentChipsAmount -= bet

	return bet, nil
}

func (p *Player) Raise(bet, over Chips) (Chips, error) {
	if bet > p.CurrentChipsAmount {
		return 0, fmt.Errorf("Wrong bet amount %d", bet)
	}

	p.CurrentChipsAmount -= bet + over

	return bet + over, nil
}

func (p *Player) Fall() (Chips, error) {
	p.PocketCards = nil
	p.Active = false

	return 0, nil
}

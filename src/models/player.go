package models

import (
	"errors"
	"fmt"
)

type Player struct {
	ID     string
	Name   string
	Active bool

	currentChipsAmount Chips
	pocketCards        []*Card
}

func (p *Player) GetCurrentChipsAmount() Chips {
	return p.currentChipsAmount
}

func (p *Player) GetPocketCards() []*Card {
	return p.pocketCards
}

func (p *Player) AddCard(card *Card) error {
	if len(p.pocketCards) == PocketSize {
		return errors.New("only to pocket cards allowed")
	}

	p.pocketCards = append(p.pocketCards, card)

	return nil
}

func (p *Player) Check(bet Chips) (Chips, error) {
	return 0, nil
}

func (p *Player) Call(bet Chips) (Chips, error) {
	if bet > p.currentChipsAmount {
		return 0, fmt.Errorf("Wrong bet amount %d", bet)
	}

	p.currentChipsAmount -= bet

	return bet, nil
}

func (p *Player) Raise(bet, over Chips) (Chips, error) {
	if bet > p.currentChipsAmount {
		return 0, fmt.Errorf("Wrong bet amount %d", bet)
	}

	p.currentChipsAmount -= bet + over

	return bet + over, nil
}

func (p *Player) Fall() (Chips, error) {
	p.pocketCards = nil
	p.Active = false

	return 0, nil
}

package models

import (
	"errors"
	"fmt"
	"hands/src/helpers"
	"sync"
)

type Player struct {
	ID     string
	Name   string
	Active bool

	currentChipsAmount Chips
	pocketCards        []*Card
	sync.RWMutex
}

func NewPlayer(name string, idMaker IdMaker, chipsAmount Chips) *Player {
	return &Player{
		ID:                 idMaker.MakeID(),
		Name:               name,
		Active:             true,
		currentChipsAmount: chipsAmount,
		pocketCards:        make([]*Card, 0),
		RWMutex:            sync.RWMutex{},
	}
}

func NewPlayerWithDefaultID(name string, chipsAmount Chips) *Player {
	return &Player{
		ID:                 helpers.NewDefaultIdGenerator().MakeID(),
		Name:               name,
		Active:             true,
		currentChipsAmount: chipsAmount,
		pocketCards:        make([]*Card, 0),
		RWMutex:            sync.RWMutex{},
	}
}

func (p *Player) GetCurrentChipsAmount() Chips {
	p.RLock()
	defer p.RUnlock()

	return p.currentChipsAmount
}

func (p *Player) GetPocketCards() []*Card {
	p.RLock()
	defer p.RUnlock()

	return p.pocketCards
}

func (p *Player) AddCard(card *Card) error {
	p.RLock()
	defer p.RUnlock()

	if len(p.pocketCards) == PocketSize {
		return errors.New("only two pocket cards allowed")
	}

	p.pocketCards = append(p.pocketCards, card)

	return nil
}

func (p *Player) Check(bet Chips) (Chips, error) {
	return 0, nil
}

func (p *Player) Call(bet Chips) (Chips, error) {
	p.RLock()
	defer p.RUnlock()

	if bet > p.currentChipsAmount {
		return 0, fmt.Errorf("Wrong bet amount %d", bet)
	}

	p.currentChipsAmount -= bet

	return bet, nil
}

func (p *Player) Raise(bet, over Chips) (Chips, error) {
	p.RLock()
	defer p.RUnlock()

	if bet > p.currentChipsAmount {
		return 0, fmt.Errorf("Wrong bet amount %d", bet)
	}

	p.currentChipsAmount -= bet + over

	return bet + over, nil
}

func (p *Player) Fall() (Chips, error) {
	p.RLock()
	defer p.RUnlock()

	p.pocketCards = nil
	p.Active = false

	return 0, nil
}

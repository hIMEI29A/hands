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
	mu                 sync.RWMutex
}

// NewPlayer creates a player with the given name and starting chips.
// A nil idMaker falls back to the default ID generator; a negative
// chipsAmount is clamped to 0.
func NewPlayer(name string, idMaker IdMaker, chipsAmount Chips) *Player {
	if idMaker == nil {
		idMaker = helpers.NewDefaultIdGenerator()
	}

	if chipsAmount < 0 {
		chipsAmount = 0
	}

	return &Player{
		ID:                 idMaker.MakeID(),
		Name:               name,
		Active:             true,
		currentChipsAmount: chipsAmount,
		pocketCards:        make([]*Card, 0),
	}
}

// NewPlayerWithDefaultID creates a player using the default ID generator.
// A negative chipsAmount is clamped to 0.
func NewPlayerWithDefaultID(name string, chipsAmount Chips) *Player {
	return NewPlayer(name, helpers.NewDefaultIdGenerator(), chipsAmount)
}

func (p *Player) GetCurrentChipsAmount() Chips {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.currentChipsAmount
}

// GetPocketCards returns a copy of the pocket cards, so callers cannot
// mutate the player's internal slice.
func (p *Player) GetPocketCards() []*Card {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cards := make([]*Card, len(p.pocketCards))
	copy(cards, p.pocketCards)

	return cards
}

func (p *Player) AddCard(card *Card) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if card == nil {
		return errors.New("card is nil")
	}

	if len(p.pocketCards) >= PocketSize {
		return fmt.Errorf("only %d pocket cards allowed", PocketSize)
	}

	p.pocketCards = append(p.pocketCards, card)

	return nil
}

// Check validates the bet without touching the player's chips.
func (p *Player) Check(bet *Bet) (Chips, error) {
	if bet == nil {
		return 0, errors.New("bet is nil")
	}

	if bet.Bet < 0 {
		return 0, fmt.Errorf("negative bet amount %d", bet.Bet)
	}

	return bet.Bet, nil
}

// Call deducts the bet from the player's stack and returns the amount.
func (p *Player) Call(bet *Bet) (Chips, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if bet == nil {
		return 0, errors.New("bet is nil")
	}

	if bet.Bet < 0 {
		return 0, fmt.Errorf("negative bet amount %d", bet.Bet)
	}

	if bet.Bet > p.currentChipsAmount {
		return 0, fmt.Errorf("bet amount %d exceeds stack %d", bet.Bet, p.currentChipsAmount)
	}

	p.currentChipsAmount -= bet.Bet

	return bet.Bet, nil
}

// Raise deducts bet.Bet+bet.Over from the player's stack and returns
// the total. The stack is never left negative.
func (p *Player) Raise(bet *Bet) (Chips, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if bet == nil {
		return 0, errors.New("bet is nil")
	}

	if bet.Bet < 0 || bet.Over < 0 {
		return 0, fmt.Errorf("negative bet values: bet %d, over %d", bet.Bet, bet.Over)
	}

	total := bet.Bet + bet.Over
	if total > p.currentChipsAmount {
		return 0, fmt.Errorf("raise amount %d exceeds stack %d", total, p.currentChipsAmount)
	}

	p.currentChipsAmount -= total

	return total, nil
}

// Fall deactivates the player, clears the pocket cards and returns the
// committed amount. Chips are not deducted here.
func (p *Player) Fall(bet *Bet) (Chips, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if bet == nil {
		return 0, errors.New("bet is nil")
	}

	p.Active = false
	p.pocketCards = make([]*Card, 0)

	return bet.Bet, nil
}

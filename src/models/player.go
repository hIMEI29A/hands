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
	streetBet          Chips
	pocketCards        []*Card
	acted              bool
	allIn              bool
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

// GetStreetBet returns how many chips the player has committed on the
// current betting street.
func (p *Player) GetStreetBet() Chips {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.streetBet
}

// IsAllIn reports whether the player has no chips left but is still in the
// hand.
func (p *Player) IsAllIn() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.allIn
}

// HasActed reports whether the player has acted since the last full raise
// on the current street.
func (p *Player) HasActed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.acted
}

// commit moves amount chips from the player's stack into the current
// street bet and updates the all-in flag. It is used by the table engine,
// which is responsible for validating the action before calling it.
func (p *Player) commit(amount Chips) (Chips, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if amount < 0 {
		return 0, fmt.Errorf("negative bet amount %d", amount)
	}

	if amount > p.currentChipsAmount {
		return 0, fmt.Errorf("bet amount %d exceeds stack %d", amount, p.currentChipsAmount)
	}

	p.currentChipsAmount -= amount
	p.streetBet += amount

	if p.currentChipsAmount == 0 {
		p.allIn = true
	}

	return amount, nil
}

// resetForStreet clears the per-street action state. The all-in flag is
// intentionally preserved for the rest of the hand.
func (p *Player) resetForStreet() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.streetBet = 0
	p.acted = false
}

func (p *Player) markActed() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.acted = true
}

func (p *Player) fold() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Active = false
	p.acted = true
}

// addChips returns chips to the player's stack (pot awards and refunds).
func (p *Player) addChips(amount Chips) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if amount < 0 {
		return fmt.Errorf("negative chip amount %d", amount)
	}

	p.currentChipsAmount += amount

	return nil
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
//
// Deprecated: use ActionCheck with Table.Act.
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
//
// Deprecated: use ActionCall with Table.Act, which also handles short
// all-in calls.
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
//
// Deprecated: use ActionRaise with Table.Act.
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

// ResetForHand clears the pocket cards and per-hand state. A player with
// chips is reactivated; a player with an empty stack sits the hand out.
// Chips are not changed.
func (p *Player) ResetForHand() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Active = p.currentChipsAmount > 0
	p.pocketCards = make([]*Card, 0)
	p.streetBet = 0
	p.acted = false
	p.allIn = false
}

// Fall deactivates the player, clears the pocket cards and returns the
// committed amount. Chips are not deducted here.
//
// Deprecated: use ActionFold with Table.Act, which keeps the folded
// player's cards for hand history.
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

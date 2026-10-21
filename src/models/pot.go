package models

import (
	"fmt"
)

// Chips - условные игровые фишки, на которые ведется игра
type Chips int

// Pot - структура, содержащая инфу о количестве фишек,
// введенных в игру (в рамках одной раздачи), и о долях, внесенных игроками
type Pot struct {
	Tag           string
	TotalChipsNum Chips
	PlayersChips  map[string]Chips
}

func NewPot(tag string) *Pot {
	return &Pot{
		Tag:           tag,
		TotalChipsNum: 0,
		PlayersChips:  make(map[string]Chips),
	}
}

// Register adds a new player to the pot with 0 chips.
// It returns an error if the player is already registered.
func (p *Pot) Register(playerID string) error {
	if _, ok := p.PlayersChips[playerID]; ok {
		return fmt.Errorf("player with ID %s is in this game already", playerID)
	}

	p.PlayersChips[playerID] = 0

	return nil
}

// AddPlayerBet adds a bet to the player's contribution and the pot total,
// and returns the player's total contribution.
func (p *Pot) AddPlayerBet(bet Chips, playerID string) (int, error) {
	if _, ok := p.PlayersChips[playerID]; !ok {
		return 0, fmt.Errorf("player with ID %s is not in this game", playerID)
	}

	if bet < 0 {
		return 0, fmt.Errorf("negative bet amount %d", bet)
	}

	p.PlayersChips[playerID] += bet
	p.TotalChipsNum += bet

	val := p.PlayersChips[playerID]

	return int(val), nil
}

// GetPlayerAmount returns the player's total contribution to the pot.
func (p *Pot) GetPlayerAmount(playerID string) (int, error) {
	if _, ok := p.PlayersChips[playerID]; !ok {
		return 0, fmt.Errorf("player with ID %s is not in this game", playerID)
	}

	val := p.PlayersChips[playerID]

	return int(val), nil
}

// Reset clears all player contributions and the pot total.
func (p *Pot) Reset() {
	p.PlayersChips = make(map[string]Chips)
	p.TotalChipsNum = 0
}

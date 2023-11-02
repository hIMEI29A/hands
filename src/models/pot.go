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

func (p *Pot) Register(playerID string) error {
	if _, ok := p.PlayersChips[playerID]; !ok {
		return fmt.Errorf("Player with ID %s is in this game already", playerID)
	}

	p.PlayersChips[playerID] = 0

	return nil
}

func (p *Pot) AddPlayerBet(bet Chips, playerID string) (int, error) {
	if _, ok := p.PlayersChips[playerID]; !ok {
		return 0, fmt.Errorf("Player with ID %s is not in this game", playerID)
	}

	p.PlayersChips[playerID] += bet
	p.TotalChipsNum += bet

	val := p.PlayersChips[playerID]

	return int(val), nil
}

func (p *Pot) GetPlayerAmount(playerID string) (int, error) {
	if _, ok := p.PlayersChips[playerID]; !ok {
		return 0, fmt.Errorf("Player with ID %s is not in this game", playerID)
	}

	val := p.PlayersChips[playerID]

	return int(val), nil
}

func (p *Pot) Reset() {
	p.PlayersChips = make(map[string]Chips)
}

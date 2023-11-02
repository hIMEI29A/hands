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

func (p *Pot) Register(playerName string) error {
	if _, ok := p.PlayersChips[playerName]; !ok {
		return fmt.Errorf("Player with name %s is in this game already", playerName)
	}

	p.PlayersChips[playerName] = 0

	return nil
}

func (p *Pot) AddPlayerBet(bet Chips, playerName string) (int, error) {
	if _, ok := p.PlayersChips[playerName]; !ok {
		return 0, fmt.Errorf("Player with name %s is not in this game", playerName)
	}

	p.PlayersChips[playerName] += bet
	p.TotalChipsNum += bet

	val := p.PlayersChips[playerName]

	return int(val), nil
}

func (p *Pot) GetPlayerAmount(playerName string) (int, error) {
	if _, ok := p.PlayersChips[playerName]; !ok {
		return 0, fmt.Errorf("Player with name %s is not in this game", playerName)
	}

	val := p.PlayersChips[playerName]

	return int(val), nil
}

func (p *Pot) Reset() {
	p.PlayersChips = make(map[string]Chips)
}

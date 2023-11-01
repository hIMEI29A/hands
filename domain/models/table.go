package models

import (
	"fmt"
	"sort"
	"sync"

	"hands/domain/helpers"
)

type TableType string

const (
	CasheTableType       TableType = "cashe"
	TournarmentTableType           = "tournarment"
)

const CardsOnFlopNumber = 3

type Table struct {
	ID                string
	Type              TableType
	Name              string
	Pot               *Pot
	Players           []*Player
	MaxPlayersNum     int
	CurrentPlayersNum int
	BigBlind          Chips
	SmallBlind        Chips
	Board             []*Card
	Dealer            int
	CurrentMove       int
	Deck              *Deck

	m sync.RWMutex
}

func NewTable(name, tag string, tableType TableType, maxPlayersNum, bb, sb int) *Table {
	return &Table{
		// ID TODO
		Type:              tableType,
		Pot:               NewPot(tag),
		Players:           make([]*Player, 0),
		MaxPlayersNum:     maxPlayersNum,
		CurrentPlayersNum: 0,
		BigBlind:          Chips(bb),
		SmallBlind:        Chips(sb),
		Deck:              NewDeck(),
		Dealer:            0,
		CurrentMove:       0,
		m:                 sync.RWMutex{},
	}
}

func (t *Table) GetPlayerByName(name string) *Player {
	for _, p := range t.Players {
		if p.Name == name {
			return p
		}
	}

	return nil
}

func (t *Table) Register(player *Player) error {
	t.m.Lock()
	defer t.m.Unlock()

	for _, p := range t.Players {
		if p.Name == player.Name {
			return fmt.Errorf("Player with name %s is in this game already", player.Name)
		}
	}

	if len(t.Players) == t.MaxPlayersNum {
		return fmt.Errorf("Players limit %d exceeded for table %s", t.MaxPlayersNum, t.Name)
	}

	t.Players = append(t.Players, player)

	return t.Pot.Register(player.Name)
}

func (t *Table) SetDealer() int {
	d := helpers.GenerateRandomNumInRange(len(t.Players))

	t.Dealer = d

	return d
}

func (t *Table) NextDealer() int {
	if t.Dealer == len(t.Players)-1 {
		t.Dealer = 0

		return 0
	}

	t.Dealer++

	return t.Dealer
}

func (t *Table) GetDealer() *Player {
	return t.Players[t.Dealer]
}

func (t *Table) GetFirstPosition() *Player {
	if t.Dealer == len(t.Players)-1 {
		return t.Players[0]
	}

	return t.Players[t.Dealer+1]
}

func (t *Table) GetSecondPosition() *Player {
	if t.Dealer == len(t.Players)-2 {
		return t.Players[0]
	}

	return t.Players[t.Dealer+2]
}

func (t *Table) GetPlayersHand(pocket []*Card) *HandRaw {
	m := GetMaxHandWithBoard(NewStringSliceFromCards(t.Board), NewStringSliceFromCards(pocket))

	return m
}

func (t *Table) ResolveWinner() string {
	playersHandsMap := make(map[string]*HandRaw)
	hands := make([]*HandRaw, 0)

	for _, player := range t.Players {
		h := t.GetPlayersHand(player.PocketCards)

		fmt.Println(h)

		hands = append(hands, h)
		playersHandsMap[player.Name] = h
	}

	sort.Slice(hands, func(i, j int) bool {
		return hands[i].Compare(hands[j]) < 0
	})

	for name, hand := range playersHandsMap {
		if hand.Same(hands[len(hands)-1]) {
			return name
		}
	}

	return ""
}

func (t *Table) PreFlop() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if err := t.Deck.Discard(); err != nil {
		return nil, err
	}

	for i := 0; i < 2; i++ {
		for _, player := range t.Players {
			card, err := t.Deck.Card()
			if err != nil {
				return nil, err
			}

			player.PocketCards = append(player.PocketCards, card)
		}
	}

	return t, nil
}

func (t *Table) Flop() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if err := t.Deck.Discard(); err != nil {
		return nil, err
	}

	for i := 0; i < CardsOnFlopNumber; i++ {
		card, err := t.Deck.Card()
		if err != nil {
			return nil, err
		}

		t.Board = append(t.Board, card)
	}

	return t, nil
}

func (t *Table) openOne() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if err := t.Deck.Discard(); err != nil {
		return nil, err
	}

	card, err := t.Deck.Card()
	if err != nil {
		return nil, err
	}

	t.Board = append(t.Board, card)

	return t, nil
}

func (t *Table) Turn() (*Table, error) {
	return t.openOne()
}

func (t *Table) River() (*Table, error) {
	return t.openOne()
}

func (t *Table) Blinds() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	small := t.GetFirstPosition()
	big := t.GetSecondPosition()

	smallBlind, err := small.Call(t.SmallBlind)
	if err != nil {
		return nil, err
	}

	if _, err = t.Pot.AddPlayerBet(smallBlind, small.Name); err != nil {
		return nil, err
	}

	bigBlind, err := big.Call(t.BigBlind)
	if err != nil {
		return nil, err
	}

	if _, err = t.Pot.AddPlayerBet(bigBlind, small.Name); err != nil {
		return nil, err
	}

	return t, nil
}

package models

import (
	"fmt"
	"hands/src/helpers"
	"sort"
	"sync"
)

type TableType string

const (
	CasheTableType       TableType = "cashe"
	TournarmentTableType           = "tournarment"
)

const CardsOnFlopNumber = 3

type TableRules struct {
	// TODO
}

type DealFunc func() (*Table, error)

type Table struct {
	ID                string
	Type              TableType
	TableRules        *TableRules
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

	deck             *Deck
	m                sync.RWMutex
	dealFuncs        []DealFunc
	isDealerInactive bool
}

func NewTable(name, tag string, tableType TableType, maxPlayersNum, bb, sb int) *Table {
	t := &Table{
		// ID TODO
		Type:              tableType,
		Name:              name,
		Pot:               NewPot(tag),
		Players:           make([]*Player, 0),
		MaxPlayersNum:     maxPlayersNum,
		CurrentPlayersNum: 0,
		BigBlind:          Chips(bb),
		SmallBlind:        Chips(sb),
		deck:              NewDeck(),
		Dealer:            0,
		CurrentMove:       0,
		m:                 sync.RWMutex{},
		isDealerInactive:  true,
	}

	t.dealFuncs = []DealFunc{
		t.PreFlop,
		t.Flop,
		t.Turn,
		t.River,
	}

	return t
}

func (t *Table) GetPlayerByID(id string) *Player {
	for _, p := range t.Players {
		if p.ID == id {
			return p
		}
	}

	return nil
}

func (t *Table) Register(player *Player) error {
	t.m.Lock()
	defer t.m.Unlock()

	for _, p := range t.Players {
		if p.ID == player.ID {
			return fmt.Errorf("Player with ID %s is in this game already", player.ID)
		}
	}

	if len(t.Players) == t.MaxPlayersNum {
		return fmt.Errorf("Players limit %d exceeded for table %s", t.MaxPlayersNum, t.Name)
	}

	t.Players = append(t.Players, player)

	return t.Pot.Register(player.ID)
}

func (t *Table) GetDealFuncs() []DealFunc {
	return t.dealFuncs
}

func (t *Table) setDealer() *Table {
	d := helpers.GenerateRandomNumInRange(len(t.Players))

	t.Dealer = d

	return t
}

func (t *Table) NextDealer() int {
	if t.isDealerInactive {
		t.isDealerInactive = false

		return t.setDealer().Dealer
	}

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

func (t *Table) GetPlayersHand(pocket []*Card) (*Hand, error) {
	return GetMaxHandWithBoard(NewStringSliceFromCards(t.Board), NewStringSliceFromCards(pocket))
}

// ResolveWinner возвращает слайс с id игроков - обладателей максимальных рук
func (t *Table) ResolveWinner() ([]string, error) {
	playersHandsMap := make(map[string]*Hand)
	hands := make([]*Hand, 0)

	for _, player := range t.Players {
		h, err := t.GetPlayersHand(player.GetPocketCards())
		if err != nil {
			return nil, err
		}

		fmt.Println(h) // TODO remove

		hands = append(hands, h)
		playersHandsMap[player.ID] = h
	}

	sort.Slice(hands, func(i, j int) bool {
		return hands[i].Compare(hands[j]) < 0
	})

	similar := make([]*Hand, 0)
	lastNum, preLastNum := len(hands)-1, len(hands)-2

	// если есть только одна максимальная рука
	if hands[lastNum].Compare(hands[preLastNum]) != 0 {
		for id, hand := range playersHandsMap {
			if hand.Same(hands[lastNum]) {
				return []string{id}, nil
			}
		}
	} else {
		result := make([]string, 0)

		for hands[lastNum].Compare(hands[preLastNum]) == 0 {
			similar = append(similar, hands[lastNum], hands[preLastNum])
			if preLastNum == 0 { // len(hands) == 2
				break
			}

			lastNum--
			preLastNum--
		}

		for id, hand := range playersHandsMap {
			for _, h := range similar {
				if hand.Same(h) {
					result = append(result, id)
				}
			}
		}

		return result, nil
	}

	return nil, nil
}

func (t *Table) PreFlop() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if err := t.deck.Discard(); err != nil {
		return nil, err
	}

	for i := 0; i < PocketSize; i++ {
		for _, player := range t.Players {
			card, err := t.deck.Card()
			if err != nil {
				return nil, err
			}

			if err := player.AddCard(card); err != nil {
				return nil, err
			}
		}
	}

	return t, nil
}

func (t *Table) Flop() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if err := t.deck.Discard(); err != nil {
		return nil, err
	}

	for i := 0; i < CardsOnFlopNumber; i++ {
		card, err := t.deck.Card()
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

	if err := t.deck.Discard(); err != nil {
		return nil, err
	}

	card, err := t.deck.Card()
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

	if _, err = t.Pot.AddPlayerBet(smallBlind, small.ID); err != nil {
		return nil, err
	}

	bigBlind, err := big.Call(t.BigBlind)
	if err != nil {
		return nil, err
	}

	if _, err = t.Pot.AddPlayerBet(bigBlind, big.ID); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Table) ShuffleDeck() *Table {
	t.m.Lock()
	defer t.m.Unlock()

	t.deck = t.deck.Shuffle()

	return t
}

func (t *Table) GetCard() (*Card, *Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	card, err := t.deck.Card()
	if err != nil {
		return nil, nil, err
	}

	return card, t, nil
}

func (t *Table) DropCard() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	err := t.deck.Discard()
	if err != nil {
		return nil, err
	}

	return t, nil
}

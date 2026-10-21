package models

import (
	"errors"
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

// BoardSize is the number of community cards on a complete board.
const BoardSize = 5

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

func NewTable(name, tag string, idMaker IdMaker, tableType TableType, maxPlayersNum, bb, sb int) *Table {
	if idMaker == nil {
		idMaker = helpers.NewDefaultIdGenerator()
	}

	return newTable(idMaker.MakeID(), name, tag, tableType, maxPlayersNum, bb, sb)
}

func NewTableWithDefaultId(name, tag string, tableType TableType, maxPlayersNum, bb, sb int) *Table {
	return newTable(helpers.NewDefaultIdGenerator().MakeID(), name, tag, tableType, maxPlayersNum, bb, sb)
}

// newTable is the single constructor shared by the exported table
// constructors; both pass only the generated ID as a difference.
func newTable(id, name, tag string, tableType TableType, maxPlayersNum, bb, sb int) *Table {
	t := &Table{
		ID:                id,
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

// getPlayerByID searches the players without taking the table lock.
// Callers must hold at least a read lock on t.m.
func (t *Table) getPlayerByID(id string) *Player {
	for _, p := range t.Players {
		if p.ID == id {
			return p
		}
	}

	return nil
}

// GetPlayerByID returns the player with the given ID or nil when absent.
func (t *Table) GetPlayerByID(id string) *Player {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.getPlayerByID(id)
}

func (t *Table) Register(player *Player) error {
	if player == nil {
		return errors.New("player is nil")
	}

	t.m.Lock()
	defer t.m.Unlock()

	if t.getPlayerByID(player.ID) != nil {
		return fmt.Errorf("player with ID %s is in this game already", player.ID)
	}

	if t.MaxPlayersNum > 0 && len(t.Players) >= t.MaxPlayersNum {
		return fmt.Errorf("players limit %d exceeded for table %s", t.MaxPlayersNum, t.Name)
	}

	// Register in the pot before appending so a pot error leaves the
	// table state untouched.
	if err := t.Pot.Register(player.ID); err != nil {
		return err
	}

	t.Players = append(t.Players, player)

	return nil
}

func (t *Table) GetDealFuncs() []DealFunc {
	return t.dealFuncs
}

// GetFirstPosition returns the player acting first after the dealer
// (the button + 1 seat, wrapping around).
func (t *Table) GetFirstPosition() (*Player, error) {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.firstPositionLocked()
}

func (t *Table) firstPositionLocked() (*Player, error) {
	if len(t.Players) == 0 {
		return nil, errors.New("table has no players")
	}

	return t.Players[(t.Dealer+1)%len(t.Players)], nil
}

// GetSecondPosition returns the player two seats after the dealer
// (wrapping around).
func (t *Table) GetSecondPosition() (*Player, error) {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.secondPositionLocked()
}

func (t *Table) secondPositionLocked() (*Player, error) {
	if len(t.Players) == 0 {
		return nil, errors.New("table has no players")
	}

	return t.Players[(t.Dealer+2)%len(t.Players)], nil
}

// GetDealer returns the player currently holding the dealer button.
func (t *Table) GetDealer() (*Player, error) {
	t.m.RLock()
	defer t.m.RUnlock()

	if len(t.Players) == 0 {
		return nil, errors.New("table has no players")
	}

	return t.Players[t.Dealer%len(t.Players)], nil
}

// nextDealerLocked moves the dealer button; requires t.m to be held.
// With no players it leaves the dealer unchanged.
func (t *Table) nextDealerLocked() {
	if len(t.Players) == 0 {
		return
	}

	if t.isDealerInactive {
		t.isDealerInactive = false

		t.Dealer = helpers.GenerateRandomNumInRange(len(t.Players))

		return
	}

	t.Dealer = (t.Dealer + 1) % len(t.Players)
}

// NextDealer moves the dealer button to the next seat (or picks a random
// seat for the first hand) and returns the new dealer index. With no
// players it returns the current index unchanged.
func (t *Table) NextDealer() int {
	t.m.Lock()
	defer t.m.Unlock()

	t.nextDealerLocked()

	return t.Dealer
}

func (t *Table) GetPlayersHand(pocket []*Card) (*Hand, error) {
	t.m.RLock()
	defer t.m.RUnlock()

	return GetMaxHandWithBoard(NewStringSliceFromCards(t.Board), NewStringSliceFromCards(pocket))
}

// StartHand prepares the table for a new hand: it clears the board,
// resets and repopulates the pot, resets every player's pocket cards
// and activity state, shuffles the deck and moves the dealer button.
// It requires at least 2 registered players.
func (t *Table) StartHand() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if len(t.Players) < 2 {
		return nil, fmt.Errorf("table needs at least 2 registered players to start a hand, got %d", len(t.Players))
	}

	t.Board = make([]*Card, 0)

	t.Pot.Reset()

	for _, p := range t.Players {
		if err := t.Pot.Register(p.ID); err != nil {
			return nil, fmt.Errorf("start hand: %w", err)
		}

		p.ResetForHand()
	}

	t.deck = t.deck.Shuffle()
	t.nextDealerLocked()

	return t, nil
}

// nextActiveIndex returns the index of the first active player at or
// after seat start (wrapping around). Requires t.m to be held.
func (t *Table) nextActiveIndex(start int) (int, error) {
	n := len(t.Players)
	if n == 0 {
		return 0, errors.New("table has no players")
	}

	for i := 0; i < n; i++ {
		idx := ((start % n) + i) % n

		if t.Players[idx].Active {
			return idx, nil
		}
	}

	return 0, errors.New("table has no active players")
}

// Blinds posts the small and big blind from the first two active seats
// after the dealer and records both bets in the pot.
func (t *Table) Blinds() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	activeNum := 0

	for _, p := range t.Players {
		if p.Active {
			activeNum++
		}
	}

	if activeNum < 2 {
		return nil, fmt.Errorf("blinds require at least 2 active players, got %d", activeNum)
	}

	var (
		smallIdx int
		err      error
	)

	if len(t.Players) == 2 {
		// Heads-up: the dealer posts the small blind (both seats are
		// active, as checked above).
		smallIdx = t.Dealer % len(t.Players)
	} else {
		smallIdx, err = t.nextActiveIndex(t.Dealer + 1)
	}

	if err != nil {
		return nil, err
	}

	bigIdx, err := t.nextActiveIndex(smallIdx + 1)
	if err != nil {
		return nil, err
	}

	small := t.Players[smallIdx]
	big := t.Players[bigIdx]

	smallBlind, err := small.Call(NewBet(t.SmallBlind, 0))
	if err != nil {
		return nil, err
	}

	if _, err = t.Pot.AddPlayerBet(smallBlind, small.ID); err != nil {
		return nil, err
	}

	bigBlind, err := big.Call(NewBet(t.BigBlind, 0))
	if err != nil {
		return nil, err
	}

	if _, err = t.Pot.AddPlayerBet(bigBlind, big.ID); err != nil {
		return nil, err
	}

	return t, nil
}

// ResolveWinner evaluates every active player's pocket cards against the
// board and returns the IDs of all players holding the maximum hand
// (split pot), sorted for deterministic output.
func (t *Table) ResolveWinner() ([]string, error) {
	t.m.RLock()
	defer t.m.RUnlock()

	if len(t.Board) != BoardSize {
		return nil, fmt.Errorf("board must contain exactly %d cards to resolve a winner, got %d", BoardSize, len(t.Board))
	}

	// Copy the board card strings so nothing can mutate t.Board.
	board := NewStringSliceFromCards(t.Board)

	var maxHand *Hand
	winners := make(map[string]*Hand, len(t.Players))
	activeNum := 0

	for _, p := range t.Players {
		if !p.Active {
			continue
		}

		activeNum++

		pocket := p.GetPocketCards()
		if len(pocket) != PocketSize {
			return nil, fmt.Errorf("player %s has %d pocket cards, want %d", p.ID, len(pocket), PocketSize)
		}

		h, err := GetMaxHandWithBoard(board, NewStringSliceFromCards(pocket))
		if err != nil {
			return nil, err
		}

		winners[p.ID] = h

		if maxHand == nil || h.Compare(maxHand) > 0 {
			maxHand = h
		}
	}

	if activeNum == 0 {
		return nil, errors.New("no active players to resolve a winner")
	}

	ids := make([]string, 0)
	for id, h := range winners {
		if maxHand.Compare(h) == 0 {
			ids = append(ids, id)
		}
	}

	sort.Strings(ids)

	return ids, nil
}

// PreFlop deals 2 pocket cards to each active player. The deck must be
// shuffled, the board must be empty and every active player must have an
// empty pocket.
func (t *Table) PreFlop() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.deck.Len() == 0 {
		return nil, errors.New("deck is not shuffled")
	}

	if len(t.Board) != 0 {
		return nil, fmt.Errorf("board must be empty before PreFlop, got %d cards", len(t.Board))
	}

	for _, p := range t.Players {
		if p.Active && len(p.GetPocketCards()) != 0 {
			return nil, fmt.Errorf("player %s already has pocket cards", p.ID)
		}
	}

	for i := 0; i < PocketSize; i++ {
		for _, player := range t.Players {
			if !player.Active {
				continue
			}

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

// openBoardCards burns 1 card and opens boardLen cards onto the board.
// Requires t.m to be held.
func (t *Table) openBoardCards(n int) (*Table, error) {
	if err := t.deck.Discard(); err != nil {
		return nil, err
	}

	for i := 0; i < n; i++ {
		card, err := t.deck.Card()
		if err != nil {
			return nil, err
		}

		t.Board = append(t.Board, card)
	}

	return t, nil
}

// Flop burns one card and opens the first 3 board cards. The board must
// be empty.
func (t *Table) Flop() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if len(t.Board) != 0 {
		return nil, fmt.Errorf("board must be empty before Flop, got %d cards", len(t.Board))
	}

	return t.openBoardCards(CardsOnFlopNumber)
}

// openOne burns one card and opens a single board card. boardLen is the
// number of cards the board must have before this street. Requires t.m
// to be held.
func (t *Table) openOne(boardLen int) (*Table, error) {
	if len(t.Board) != boardLen {
		return nil, fmt.Errorf("board must contain exactly %d cards before this street, got %d", boardLen, len(t.Board))
	}

	return t.openBoardCards(1)
}

// Turn burns one card and opens the 4th board card.
func (t *Table) Turn() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	return t.openOne(3)
}

// River burns one card and opens the 5th board card.
func (t *Table) River() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	return t.openOne(4)
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

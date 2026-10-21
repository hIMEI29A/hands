package models

import (
	"errors"
	"fmt"
	"hands/src/helpers"
	"sort"
	"sync"
)

// TableType is the kind of a table.
type TableType string

// Table types.
const (
	// CashTableType is a table where the chips have cash value.
	CashTableType TableType = "cash"

	// TournamentTableType is a table where the chips have tournament value.
	TournamentTableType TableType = "tournament"
)

// Deprecated table types, kept for backward compatibility.
const (
	// CasheTableType is a table where the chips have cash value.
	//
	// Deprecated: use CashTableType instead.
	CasheTableType TableType = "cashe"

	// TournarmentTableType is a table where the chips have tournament value.
	//
	// Deprecated: use TournamentTableType instead.
	TournarmentTableType TableType = "tournarment"
)

const CardsOnFlopNumber = 3

// BoardSize is the number of community cards on a complete board.
const BoardSize = 5

type DealFunc func() (*Table, error)

type Table struct {
	ID                string
	Type              TableType
	Name              string
	Pot               *Pot
	Players           []*Player
	MaxPlayersNum     int
	CurrentPlayersNum int
	Board             []*Card
	Dealer            int

	rules            TableRules
	phase            HandPhase
	street           Street
	currentActor     int
	currentBet       Chips
	lastFullRaise    Chips
	lastFullRaiseBet Chips
	raisesThisStreet int
	lastResults      []PotResult

	deck             *Deck
	m                sync.RWMutex
	dealFuncs        []DealFunc
	isDealerInactive bool
}

func NewTable(name, tag string, idMaker IdMaker, tableType TableType, maxPlayersNum, bb, sb int) *Table {
	if idMaker == nil {
		idMaker = helpers.NewDefaultIdGenerator()
	}

	return newTable(idMaker.MakeID(), name, tag, tableType, maxPlayersNum, DefaultTableRules(Chips(sb), Chips(bb)))
}

func NewTableWithDefaultId(name, tag string, tableType TableType, maxPlayersNum, bb, sb int) *Table {
	return newTable(helpers.NewDefaultIdGenerator().MakeID(), name, tag, tableType, maxPlayersNum, DefaultTableRules(Chips(sb), Chips(bb)))
}

// NewTableWithRules creates a table with explicit rules. The rules are
// normalized and validated before the table is built.
func NewTableWithRules(name, tag string, idMaker IdMaker, tableType TableType, maxPlayersNum int, rules *TableRules) (*Table, error) {
	if idMaker == nil {
		idMaker = helpers.NewDefaultIdGenerator()
	}

	normalized := rules.normalized()
	if err := normalized.Validate(); err != nil {
		return nil, fmt.Errorf("invalid table rules: %w", err)
	}

	return newTable(idMaker.MakeID(), name, tag, tableType, maxPlayersNum, normalized), nil
}

// newTable is the single constructor shared by the exported table
// constructors; both pass only the generated ID as a difference.
func newTable(id, name, tag string, tableType TableType, maxPlayersNum int, rules *TableRules) *Table {
	t := &Table{
		ID:                id,
		Type:              tableType,
		Name:              name,
		Pot:               NewPot(tag),
		Players:           make([]*Player, 0),
		MaxPlayersNum:     maxPlayersNum,
		CurrentPlayersNum: 0,
		rules:             *rules.normalized(),
		deck:              NewDeck(),
		Dealer:            0,
		phase:             PhaseWaiting,
		street:            StreetPreFlop,
		currentActor:      -1,
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

// Rules returns a copy of the table rules.
func (t *Table) Rules() TableRules {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.rules
}

// SmallBlind returns the small blind amount.
func (t *Table) SmallBlind() Chips {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.rules.SmallBlind
}

// BigBlind returns the big blind amount.
func (t *Table) BigBlind() Chips {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.rules.BigBlind
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

// nextDealerLocked moves the dealer button to the next player who is in
// the hand. On the first hand the button starts at a random in-hand seat.
// Requires t.m to be held.
func (t *Table) nextDealerLocked() {
	if len(t.Players) == 0 {
		return
	}

	active := t.activeIndexesLocked()
	if len(active) == 0 {
		return
	}

	if t.isDealerInactive {
		t.isDealerInactive = false
		t.Dealer = active[helpers.GenerateRandomNumInRange(len(active))]

		return
	}

	idx, err := t.nextActiveIndex(t.Dealer + 1)
	if err != nil {
		return
	}

	t.Dealer = idx
}

// activeIndexesLocked returns the seat indexes of all players currently in
// the hand, in seat order.
func (t *Table) activeIndexesLocked() []int {
	active := make([]int, 0, len(t.Players))

	for i, p := range t.Players {
		if p.Active {
			active = append(active, i)
		}
	}

	return active
}

func (t *Table) activeCountLocked() int {
	count := 0

	for _, p := range t.Players {
		if p.Active {
			count++
		}
	}

	return count
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

// StartHand prepares and starts a complete hand: it resets per-hand
// state, sits out players with empty stacks, shuffles the deck, moves the
// dealer button, posts blinds, deals pocket cards and opens preflop
// betting.
func (t *Table) StartHand() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if err := t.rules.Validate(); err != nil {
		return nil, fmt.Errorf("start hand: %w", err)
	}

	funded := 0
	for _, p := range t.Players {
		if p.GetCurrentChipsAmount() > 0 {
			funded++
		}
	}

	if funded < t.rules.MinPlayers {
		return nil, fmt.Errorf("table needs at least %d players with chips to start a hand, got %d", t.rules.MinPlayers, funded)
	}

	for _, p := range t.Players {
		p.ResetForHand()
	}

	active := t.activeIndexesLocked()

	t.Board = make([]*Card, 0)
	t.Pot.Reset()
	t.lastResults = nil
	t.CurrentPlayersNum = len(active)

	for _, idx := range active {
		if err := t.Pot.Register(t.Players[idx].ID); err != nil {
			return nil, fmt.Errorf("start hand: %w", err)
		}
	}

	t.deck = t.deck.Shuffle()
	t.nextDealerLocked()

	t.phase = PhaseBetting
	t.street = StreetPreFlop
	t.currentBet = 0
	t.lastFullRaise = t.rules.MinRaise
	t.lastFullRaiseBet = 0
	t.raisesThisStreet = 0
	t.currentActor = -1

	_, bigBlindIdx, err := t.postBlindsLocked()
	if err != nil {
		return nil, err
	}

	if err := t.dealPocketsLocked(); err != nil {
		return nil, err
	}

	t.currentActor = t.firstPreFlopActorLocked(bigBlindIdx)
	if t.currentActor < 0 {
		t.phase = PhaseStreetComplete
	}

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

// Blinds posts the small and big blind. It is a low-level helper kept for
// manual dealing; StartHand posts the blinds itself. Calling it after
// blinds have already been posted returns an error.
//
// Deprecated: use StartHand, which sets up the complete hand and betting
// state.
func (t *Table) Blinds() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.currentBet > 0 {
		return nil, errors.New("blinds have already been posted")
	}

	for _, p := range t.Players {
		if p.GetStreetBet() > 0 {
			return nil, errors.New("blinds have already been posted")
		}
	}

	if _, _, err := t.postBlindsLocked(); err != nil {
		return nil, err
	}

	return t, nil
}

// postBlindsLocked posts the small and big blind for the current hand and
// initializes the preflop bet size. Requires t.m to be held.
func (t *Table) postBlindsLocked() (int, int, error) {
	activeNum := t.activeCountLocked()
	if activeNum < 2 {
		return 0, 0, fmt.Errorf("blinds require at least 2 active players, got %d", activeNum)
	}

	var (
		smallIdx int
		err      error
	)

	if activeNum == 2 {
		// Heads-up: the dealer posts the small blind.
		smallIdx = t.Dealer
		if !t.Players[smallIdx].Active {
			smallIdx, err = t.nextActiveIndex(t.Dealer + 1)
			if err != nil {
				return 0, 0, err
			}
		}
	} else {
		smallIdx, err = t.nextActiveIndex(t.Dealer + 1)
		if err != nil {
			return 0, 0, err
		}
	}

	bigIdx, err := t.nextActiveIndex(smallIdx + 1)
	if err != nil {
		return 0, 0, err
	}

	if err := t.postBlindLocked(t.Players[smallIdx], t.rules.SmallBlind); err != nil {
		return 0, 0, err
	}

	if err := t.postBlindLocked(t.Players[bigIdx], t.rules.BigBlind); err != nil {
		return 0, 0, err
	}

	t.currentBet = t.rules.BigBlind
	t.lastFullRaise = t.rules.MinRaise
	t.lastFullRaiseBet = t.rules.BigBlind
	t.raisesThisStreet = 0

	return smallIdx, bigIdx, nil
}

// postBlindLocked deducts min(blind, stack) from the player and records it
// in the pot. Requires t.m to be held.
func (t *Table) postBlindLocked(p *Player, blind Chips) error {
	amount := blind
	if stack := p.GetCurrentChipsAmount(); stack < amount {
		amount = stack
	}

	if amount <= 0 {
		return nil
	}

	paid, err := p.commit(amount)
	if err != nil {
		return err
	}

	if _, err := t.Pot.AddPlayerBet(paid, p.ID); err != nil {
		return err
	}

	return nil
}

// ResolveWinner evaluates every active player's pocket cards against the
// board and returns the IDs of all players holding the maximum hand
// (split pot), sorted for deterministic output. It does not build side
// pots or move chips.
//
// Deprecated: use ResolvePots or Showdown for pot-aware results.
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
// empty pocket. It bypasses the betting state machine.
//
// Deprecated: StartHand deals pocket cards and opens preflop betting.
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
// be empty. It bypasses the betting state machine.
//
// Deprecated: use NextStreet after a completed preflop betting round.
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

// Turn burns one card and opens the 4th board card. It bypasses the
// betting state machine.
//
// Deprecated: use NextStreet after a completed flop betting round.
func (t *Table) Turn() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	return t.openOne(3)
}

// River burns one card and opens the 5th board card. It bypasses the
// betting state machine.
//
// Deprecated: use NextStreet after a completed turn betting round.
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

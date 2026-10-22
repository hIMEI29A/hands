// Package tournament implements a single-table freezeout sit-and-go
// tournament on top of the engine package.
//
// A tournament runs one engine.Table from 2 to MaxPlayers players. Blinds
// advance by completed hands, busted players receive a final place and the
// last remaining player wins. Prizes are abstract units, not tournament
// chips; the engine only moves chips between players.
package tournament

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"hands/src/engine"
)

// State is the lifecycle state of a tournament.
type State string

// Tournament states.
const (
	StateRegistering State = "registering"
	StateRunning     State = "running"
	StateFinished    State = "finished"
)

// BlindLevel describes the blinds and ante for a number of completed hands.
type BlindLevel struct {
	SmallBlind engine.Chips
	BigBlind   engine.Chips
	Ante       engine.Chips
	AnteMode   engine.AnteMode
	Hands      int // completed hands on this level; must be > 0
}

// Payout is a prize for a finishing place. Amount is an abstract prize
// unit, not tournament chips.
type Payout struct {
	Place  int
	Amount int
}

// TournamentRules configures a single-table freezeout tournament.
type TournamentRules struct {
	StartingStack engine.Chips
	MinPlayers    int
	MaxPlayers    int
	Levels        []BlindLevel
	Payouts       []Payout
}

// Standing is a point-in-time view of one tournament entry.
type Standing struct {
	PlayerID   string
	Name       string
	Chips      engine.Chips
	Place      int
	Prize      int
	Eliminated bool
}

// entry is the tournament-side record of a registered player.
type entry struct {
	player     *engine.Player
	name       string
	order      int
	place      int
	prize      int
	eliminated bool
	startStack engine.Chips
}

// Tournament runs a single-table freezeout sit-and-go.
type Tournament struct {
	name    string
	tag     string
	idMaker engine.IdMaker
	rules   TournamentRules

	entries []*entry
	table   *engine.Table

	state        State
	level        int
	handsInLevel int
	handsPlayed  int

	mu sync.RWMutex
}

// NewTournament validates and normalizes the rules and creates a tournament
// in StateRegistering. A nil rules value is rejected.
func NewTournament(name, tag string, idMaker engine.IdMaker, rules *TournamentRules) (*Tournament, error) {
	normalized, err := normalizeRules(rules)
	if err != nil {
		return nil, err
	}

	return &Tournament{
		name:    name,
		tag:     tag,
		idMaker: idMaker,
		rules:   normalized,
		state:   StateRegistering,
	}, nil
}

// Rules returns a deep copy of the tournament rules.
func (t *Tournament) Rules() TournamentRules {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return cloneRules(t.rules)
}

// Register creates a player with the starting stack and adds them to the
// tournament. Registration is only possible in StateRegistering.
func (t *Tournament) Register(name string) (*engine.Player, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != StateRegistering {
		return nil, fmt.Errorf("cannot register in state %s", t.state)
	}

	if len(t.entries) >= t.rules.MaxPlayers {
		return nil, fmt.Errorf("player limit %d reached", t.rules.MaxPlayers)
	}

	player := engine.NewPlayer(name, t.idMaker, t.rules.StartingStack)

	for _, e := range t.entries {
		if e.player.ID == player.ID {
			return nil, fmt.Errorf("player with ID %s is already registered", player.ID)
		}
	}

	t.entries = append(t.entries, &entry{
		player: player,
		name:   name,
		order:  len(t.entries),
	})

	return player, nil
}

// Start builds the engine table and moves the tournament to StateRunning.
func (t *Tournament) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != StateRegistering {
		return fmt.Errorf("cannot start from state %s", t.state)
	}

	if len(t.entries) < t.rules.MinPlayers {
		return fmt.Errorf("need at least %d players to start, got %d", t.rules.MinPlayers, len(t.entries))
	}

	if len(t.entries) > t.rules.MaxPlayers {
		return fmt.Errorf("player limit %d exceeded", t.rules.MaxPlayers)
	}

	if len(t.rules.Payouts) > len(t.entries) {
		return fmt.Errorf("cannot fill %d payout places with %d players", len(t.rules.Payouts), len(t.entries))
	}

	table, err := engine.NewTableWithRules(
		t.name,
		t.tag,
		t.idMaker,
		engine.TournamentTableType,
		t.rules.MaxPlayers,
		rulesForLevel(t.rules.Levels[0]),
	)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	for _, e := range t.entries {
		if err := table.Register(e.player); err != nil {
			return fmt.Errorf("register player %s: %w", e.player.ID, err)
		}
	}

	t.table = table
	t.state = StateRunning
	t.level = 0
	t.handsInLevel = 0
	t.handsPlayed = 0

	return nil
}

// StartHand applies the current level rules and starts a new hand. It is
// only possible in StateRunning and only between hands.
func (t *Tournament) StartHand() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != StateRunning {
		return fmt.Errorf("cannot start a hand in state %s", t.state)
	}

	if t.table == nil {
		return errors.New("tournament table is not initialized")
	}

	t.advanceLevelIfDueLocked()

	if err := t.table.SetRules(rulesForLevel(t.rules.Levels[t.level])); err != nil {
		return err
	}

	for _, e := range t.entries {
		if e.eliminated {
			continue
		}

		e.startStack = e.player.GetCurrentChipsAmount()
	}

	if _, err := t.table.StartHand(); err != nil {
		return fmt.Errorf("start hand: %w", err)
	}

	return nil
}

// Act delegates a player action to the engine table.
func (t *Tournament) Act(action engine.Action) (engine.ActionResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.state != StateRunning {
		return engine.ActionResult{}, fmt.Errorf("cannot act in state %s", t.state)
	}

	if t.table == nil {
		return engine.ActionResult{}, errors.New("tournament table is not initialized")
	}

	return t.table.Act(action)
}

// NextStreet advances the engine table to the next street.
func (t *Tournament) NextStreet() error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.state != StateRunning {
		return fmt.Errorf("cannot advance a street in state %s", t.state)
	}

	if t.table == nil {
		return errors.New("tournament table is not initialized")
	}

	_, err := t.table.NextStreet()

	return err
}

// Showdown resolves the current hand, records eliminations and places and
// advances the blind level when its hand count is reached.
func (t *Tournament) Showdown() ([]engine.PotResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != StateRunning {
		return nil, fmt.Errorf("cannot show down in state %s", t.state)
	}

	if t.table == nil {
		return nil, errors.New("tournament table is not initialized")
	}

	results, err := t.table.Showdown()
	if err != nil {
		return nil, err
	}

	t.handsPlayed++
	t.handsInLevel++

	remainingBefore := 0

	for _, e := range t.entries {
		if !e.eliminated {
			remainingBefore++
		}
	}

	busted := make([]*entry, 0)

	for _, e := range t.entries {
		if e.eliminated {
			continue
		}

		if e.player.GetCurrentChipsAmount() == 0 {
			busted = append(busted, e)
		}
	}

	assignBustedPlaces(busted, remainingBefore)

	remaining := 0

	var survivor *entry

	for _, e := range t.entries {
		if !e.eliminated {
			remaining++
			survivor = e
		}
	}

	switch remaining {
	case 0:
		return nil, errors.New("all players have been eliminated")
	case 1:
		survivor.place = 1
		t.state = StateFinished
		t.awardPrizesLocked()
	default:
		if t.handsInLevel >= t.rules.Levels[t.level].Hands && t.level < len(t.rules.Levels)-1 {
			t.level++
			t.handsInLevel = 0
		}
	}

	return results, nil
}

// Standings returns the current tournament standings. Finished tournaments
// are ordered by place; running tournaments list funded players first.
func (t *Tournament) Standings() []Standing {
	t.mu.RLock()
	defer t.mu.RUnlock()

	standings := make([]Standing, 0, len(t.entries))
	regOrder := make(map[string]int, len(t.entries))

	for i, e := range t.entries {
		regOrder[e.player.ID] = i

		standings = append(standings, Standing{
			PlayerID:   e.player.ID,
			Name:       e.name,
			Chips:      e.player.GetCurrentChipsAmount(),
			Place:      e.place,
			Prize:      e.prize,
			Eliminated: e.eliminated,
		})
	}

	if t.state == StateFinished {
		seatOrder := buildSeatOrder(t.table)

		sort.SliceStable(standings, func(i, j int) bool {
			if standings[i].Place != standings[j].Place {
				return standings[i].Place < standings[j].Place
			}

			pi := standingsPosition(standings[i].PlayerID, seatOrder, regOrder)
			pj := standingsPosition(standings[j].PlayerID, seatOrder, regOrder)

			if pi != pj {
				return pi < pj
			}

			return standings[i].PlayerID < standings[j].PlayerID
		})

		return standings
	}

	sort.SliceStable(standings, func(i, j int) bool {
		if standings[i].Eliminated != standings[j].Eliminated {
			return !standings[i].Eliminated
		}

		if !standings[i].Eliminated {
			if standings[i].Chips != standings[j].Chips {
				return standings[i].Chips > standings[j].Chips
			}

			return standings[i].PlayerID < standings[j].PlayerID
		}

		if standings[i].Place != standings[j].Place {
			return standings[i].Place < standings[j].Place
		}

		return standings[i].PlayerID < standings[j].PlayerID
	})

	return standings
}

// State returns the tournament lifecycle state.
func (t *Tournament) State() State {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.state
}

// Level returns the current blind level index, 0-based.
func (t *Tournament) Level() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.level
}

// HandsInLevel returns the number of completed hands on the current level.
func (t *Tournament) HandsInLevel() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.handsInLevel
}

// HandsPlayed returns the total number of completed hands.
func (t *Tournament) HandsPlayed() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.handsPlayed
}

// Table returns the underlying engine table, or nil before Start. The table
// is read-only by convention.
func (t *Tournament) Table() *engine.Table {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.table
}

// advanceLevelIfDueLocked advances to the next level when the current level
// has met its hand count. The last level repeats indefinitely. Requires the
// write lock.
func (t *Tournament) advanceLevelIfDueLocked() {
	if t.level < 0 || t.level >= len(t.rules.Levels) {
		return
	}

	if t.handsInLevel < t.rules.Levels[t.level].Hands {
		return
	}

	if t.level >= len(t.rules.Levels)-1 {
		return
	}

	t.level++
	t.handsInLevel = 0
}

// awardPrizesLocked splits every place group's prize among its tied
// entries. Requires the write lock.
func (t *Tournament) awardPrizesLocked() {
	prizes := splitPrizes(t.entries, t.rules.Payouts, buildSeatOrder(t.table))
	for e, prize := range prizes {
		e.prize = prize
	}
}

// assignBustedPlaces assigns places to the entries eliminated in one hand.
// remainingBefore is the number of entries still in the tournament before
// this hand's busts. Entries with a larger start-of-hand stack receive the
// better place; equal stacks share the best place of their tie group.
func assignBustedPlaces(busted []*entry, remainingBefore int) {
	if len(busted) == 0 {
		return
	}

	sort.SliceStable(busted, func(i, j int) bool {
		return busted[i].startStack > busted[j].startStack
	})

	nextPlace := remainingBefore - len(busted) + 1

	for i := 0; i < len(busted); {
		j := i + 1
		for j < len(busted) && busted[j].startStack == busted[i].startStack {
			j++
		}

		for k := i; k < j; k++ {
			busted[k].place = nextPlace
			busted[k].eliminated = true
		}

		nextPlace += j - i
		i = j
	}
}

// splitPrizes groups the entries by place and splits the payout places
// consumed by each group. A group at place p with k entries consumes places
// p through p+k-1. Remainder units go to tied entries in seat order.
func splitPrizes(entries []*entry, payouts []Payout, seatOrder map[string]int) map[*entry]int {
	amounts := make(map[int]int, len(payouts))
	for _, payout := range payouts {
		amounts[payout.Place] = payout.Amount
	}

	groups := make(map[int][]*entry)
	for _, e := range entries {
		groups[e.place] = append(groups[e.place], e)
	}

	prizes := make(map[*entry]int, len(entries))

	for place, group := range groups {
		total := 0

		for i := 0; i < len(group); i++ {
			total += amounts[place+i]
		}

		base := total / len(group)
		remainder := total % len(group)

		for _, e := range group {
			prizes[e] = base
		}

		ordered := append([]*entry(nil), group...)

		sort.SliceStable(ordered, func(i, j int) bool {
			return seatPosition(ordered[i], seatOrder) < seatPosition(ordered[j], seatOrder)
		})

		for i := 0; i < remainder; i++ {
			prizes[ordered[i]]++
		}
	}

	return prizes
}

// seatPosition returns a stable ordering key for a tied entry. Players at
// the table are ordered clockwise from the dealer; unknown players fall back
// to registration order after the table seats.
func seatPosition(e *entry, seatOrder map[string]int) int {
	if seatOrder != nil {
		if position, ok := seatOrder[e.player.ID]; ok {
			return position
		}
	}

	return len(seatOrder) + e.order
}

// buildSeatOrder maps player IDs to their position clockwise from the
// dealer. It returns nil without a table or players.
func buildSeatOrder(table *engine.Table) map[string]int {
	if table == nil || len(table.Players) == 0 {
		return nil
	}

	n := len(table.Players)
	order := make(map[string]int, n)

	for i := 0; i < n; i++ {
		index := ((table.Dealer+1+i)%n + n) % n
		order[table.Players[index].ID] = i
	}

	return order
}

// standingsPosition returns the seat or registration position of a player.
func standingsPosition(id string, seatOrder, regOrder map[string]int) int {
	if seatOrder != nil {
		if position, ok := seatOrder[id]; ok {
			return position
		}
	}

	return len(seatOrder) + regOrder[id]
}

// normalizeRules validates the rules and returns a normalized deep copy.
func normalizeRules(rules *TournamentRules) (TournamentRules, error) {
	if rules == nil {
		return TournamentRules{}, errors.New("tournament rules are nil")
	}

	normalized := TournamentRules{
		StartingStack: rules.StartingStack,
		MinPlayers:    rules.MinPlayers,
		MaxPlayers:    rules.MaxPlayers,
		Levels:        append([]BlindLevel(nil), rules.Levels...),
		Payouts:       append([]Payout(nil), rules.Payouts...),
	}

	if normalized.MinPlayers == 0 {
		normalized.MinPlayers = 2
	}

	if normalized.MaxPlayers == 0 {
		normalized.MaxPlayers = 9
	}

	if normalized.StartingStack <= 0 {
		return TournamentRules{}, fmt.Errorf("starting stack must be positive, got %d", normalized.StartingStack)
	}

	if normalized.MinPlayers < 2 {
		return TournamentRules{}, fmt.Errorf("min players must be at least 2, got %d", normalized.MinPlayers)
	}

	if normalized.MaxPlayers < normalized.MinPlayers {
		return TournamentRules{}, fmt.Errorf(
			"max players %d must not be smaller than min players %d",
			normalized.MaxPlayers,
			normalized.MinPlayers,
		)
	}

	if len(normalized.Levels) == 0 {
		return TournamentRules{}, errors.New("at least one blind level is required")
	}

	for i, level := range normalized.Levels {
		if level.Hands <= 0 {
			return TournamentRules{}, fmt.Errorf("blind level %d must have a positive number of hands, got %d", i, level.Hands)
		}

		if err := rulesForLevel(level).Validate(); err != nil {
			return TournamentRules{}, fmt.Errorf("invalid blind level %d: %w", i, err)
		}
	}

	if len(normalized.Payouts) == 0 {
		return TournamentRules{}, errors.New("at least one payout is required")
	}

	sort.Slice(normalized.Payouts, func(i, j int) bool {
		return normalized.Payouts[i].Place < normalized.Payouts[j].Place
	})

	seen := make(map[int]bool, len(normalized.Payouts))

	for i, payout := range normalized.Payouts {
		if payout.Place <= 0 {
			return TournamentRules{}, fmt.Errorf("payout place must be positive, got %d", payout.Place)
		}

		if payout.Amount <= 0 {
			return TournamentRules{}, fmt.Errorf("payout amount for place %d must be positive, got %d", payout.Place, payout.Amount)
		}

		if seen[payout.Place] {
			return TournamentRules{}, fmt.Errorf("duplicate payout place %d", payout.Place)
		}

		seen[payout.Place] = true

		if payout.Place != i+1 {
			return TournamentRules{}, fmt.Errorf("payout places must be contiguous starting at 1, got %d at index %d", payout.Place, i)
		}
	}

	if len(normalized.Payouts) > normalized.MaxPlayers {
		return TournamentRules{}, fmt.Errorf(
			"%d payout places exceed the maximum of %d players",
			len(normalized.Payouts),
			normalized.MaxPlayers,
		)
	}

	return normalized, nil
}

// rulesForLevel builds table rules for a blind level.
func rulesForLevel(level BlindLevel) *engine.TableRules {
	rules := engine.DefaultTableRules(level.SmallBlind, level.BigBlind)
	rules.Ante = level.Ante

	if level.AnteMode != "" {
		rules.AnteMode = level.AnteMode
	}

	return rules
}

// cloneRules returns a deep copy of the rules.
func cloneRules(rules TournamentRules) TournamentRules {
	rules.Levels = append([]BlindLevel(nil), rules.Levels...)
	rules.Payouts = append([]Payout(nil), rules.Payouts...)

	return rules
}

// Package tournament implements a single-table freezeout sit-and-go
// tournament on top of the engine package.
//
// A tournament runs one engine.Table from 2 to MaxPlayers players. Blinds
// advance by completed hands, busted players receive a final place and the
// last remaining player wins. Prize accounting (buy-in, fee, prize pool and
// payouts) uses abstract integer units, not tournament chips; the engine
// only moves chips between players.
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

// PayoutMode selects how tournament prizes are calculated.
type PayoutMode string

// Payout modes.
const (
	// PayoutModeFixed uses TournamentRules.Payouts as absolute prize units.
	PayoutModeFixed PayoutMode = "fixed"
	// PayoutModePercent uses TournamentRules.PercentPayouts as percentages
	// of the prize pool. Percentages must sum to 100.
	PayoutModePercent PayoutMode = "percent"
	// PayoutModeWinnerTakeAll awards the whole prize pool to first place.
	PayoutModeWinnerTakeAll PayoutMode = "winner-take-all"
)

// PercentPayout assigns a whole percentage of the prize pool to a place.
type PercentPayout struct {
	Place   int
	Percent int
}

// TournamentRules configures a single-table freezeout tournament.
//
// BuyIn and Fee are abstract units per entry; Fee is not part of the prize
// pool. The prize pool is max(BuyIn*entries, Guarantee). With no BuyIn and no
// Guarantee, fixed Payouts remain absolute prize units for backward
// compatibility and the prize pool is zero. PayoutMode selects the payout
// table; an empty mode means PayoutModeFixed. PercentPayouts is used only in
// PayoutModePercent.
type TournamentRules struct {
	StartingStack  engine.Chips
	MinPlayers     int
	MaxPlayers     int
	Levels         []BlindLevel
	Payouts        []Payout
	BuyIn          int
	Fee            int
	Guarantee      int
	PayoutMode     PayoutMode
	PercentPayouts []PercentPayout
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

// PrizePool returns the current prize pool in abstract prize units. It is
// max(BuyIn * registered entries, Guarantee). In legacy fixed-payout mode
// without a BuyIn or Guarantee the pool is zero. A zero pool can also mean a
// funded tournament with no entries yet.
func (t *Tournament) PrizePool() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.prizePoolLocked()
}

// EntryCost returns BuyIn plus Fee for one entry.
func (t *Tournament) EntryCost() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.rules.BuyIn + t.rules.Fee
}

// FeesCollected returns the total fees for all registered entries. Fees are
// not part of the prize pool.
func (t *Tournament) FeesCollected() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.rules.Fee * len(t.entries)
}

// prizePoolLocked computes the prize pool. Requires at least a read lock.
func (t *Tournament) prizePoolLocked() int {
	pool := t.rules.BuyIn * len(t.entries)
	if t.rules.Guarantee > pool {
		pool = t.rules.Guarantee
	}

	return pool
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

	places := t.rules.payoutPlaces()
	if places > len(t.entries) {
		return fmt.Errorf("cannot fill %d payout places with %d players", places, len(t.entries))
	}

	pool := t.prizePoolLocked()

	switch t.rules.PayoutMode {
	case PayoutModeFixed:
		if t.rules.accountingEnabled() {
			total := 0
			for _, payout := range t.rules.Payouts {
				total += payout.Amount
			}

			if total > pool {
				return fmt.Errorf("payout total %d exceeds prize pool %d", total, pool)
			}
		}
	case PayoutModePercent, PayoutModeWinnerTakeAll:
		if pool <= 0 {
			return fmt.Errorf("prize pool must be positive in payout mode %s, got %d", t.rules.PayoutMode, pool)
		}
	default:
		return fmt.Errorf("unknown payout mode %q", t.rules.PayoutMode)
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
	var amounts map[int]int

	switch t.rules.PayoutMode {
	case PayoutModePercent:
		amounts = percentPrizeAmounts(t.prizePoolLocked(), t.rules.PercentPayouts)
	case PayoutModeWinnerTakeAll:
		amounts = map[int]int{1: t.prizePoolLocked()}
	default:
		amounts = fixedPrizeAmounts(t.rules.Payouts)
	}

	prizes := splitPrizeAmounts(t.entries, amounts, buildSeatOrder(t.table))
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
// p through p+k-1. Remainder units go to tied entries in seat order. It is
// kept as a fixed-payout test seam; production code uses splitPrizeAmounts.
func splitPrizes(entries []*entry, payouts []Payout, seatOrder map[string]int) map[*entry]int {
	return splitPrizeAmounts(entries, fixedPrizeAmounts(payouts), seatOrder)
}

// fixedPrizeAmounts indexes fixed payouts by place.
func fixedPrizeAmounts(payouts []Payout) map[int]int {
	amounts := make(map[int]int, len(payouts))
	for _, payout := range payouts {
		amounts[payout.Place] = payout.Amount
	}

	return amounts
}

// percentPrizeAmounts converts percentages of the pool into whole prize
// units. Each place is floored; the leftover units go to the largest
// fractional remainders, with equal remainders favouring the better place
// (the lower place number).
func percentPrizeAmounts(pool int, payouts []PercentPayout) map[int]int {
	amounts := make(map[int]int, len(payouts))
	remainders := make(map[int]int, len(payouts))
	places := make([]int, 0, len(payouts))
	totalPercent := 0
	distributed := 0

	for _, payout := range payouts {
		// Splitting the pool keeps pool*percent within the int range for
		// realistic prize pools.
		base := (pool/100)*payout.Percent + (pool%100)*payout.Percent/100
		amounts[payout.Place] = base
		remainders[payout.Place] = ((pool % 100) * payout.Percent) % 100
		places = append(places, payout.Place)
		totalPercent += payout.Percent
		distributed += base
	}

	target := (pool/100)*totalPercent + (pool%100)*totalPercent/100
	leftover := target - distributed

	sort.SliceStable(places, func(i, j int) bool {
		if remainders[places[i]] != remainders[places[j]] {
			return remainders[places[i]] > remainders[places[j]]
		}

		return places[i] < places[j]
	})

	if leftover > len(places) {
		leftover = len(places)
	}

	for i := 0; i < leftover; i++ {
		amounts[places[i]]++
	}

	return amounts
}

// splitPrizeAmounts groups the entries by place and splits the prize amounts
// of the places consumed by each group. A group at place p with k entries
// consumes places p through p+k-1. Remainder units go to tied entries in
// seat order.
func splitPrizeAmounts(entries []*entry, amounts map[int]int, seatOrder map[string]int) map[*entry]int {
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
		StartingStack:  rules.StartingStack,
		MinPlayers:     rules.MinPlayers,
		MaxPlayers:     rules.MaxPlayers,
		Levels:         append([]BlindLevel(nil), rules.Levels...),
		Payouts:        append([]Payout(nil), rules.Payouts...),
		BuyIn:          rules.BuyIn,
		Fee:            rules.Fee,
		Guarantee:      rules.Guarantee,
		PayoutMode:     rules.PayoutMode,
		PercentPayouts: append([]PercentPayout(nil), rules.PercentPayouts...),
	}

	if normalized.PayoutMode == "" {
		normalized.PayoutMode = PayoutModeFixed
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

	if normalized.BuyIn < 0 {
		return TournamentRules{}, fmt.Errorf("buy-in must not be negative, got %d", normalized.BuyIn)
	}

	if normalized.Fee < 0 {
		return TournamentRules{}, fmt.Errorf("fee must not be negative, got %d", normalized.Fee)
	}

	if normalized.Guarantee < 0 {
		return TournamentRules{}, fmt.Errorf("guarantee must not be negative, got %d", normalized.Guarantee)
	}

	switch normalized.PayoutMode {
	case PayoutModeFixed:
		if len(normalized.PercentPayouts) > 0 {
			return TournamentRules{}, errors.New("percent payouts are not allowed in fixed payout mode")
		}

		if err := validateFixedPayouts(&normalized); err != nil {
			return TournamentRules{}, err
		}
	case PayoutModePercent:
		if len(normalized.Payouts) > 0 {
			return TournamentRules{}, errors.New("fixed payouts are not allowed in percent payout mode")
		}

		if err := validatePercentPayouts(&normalized); err != nil {
			return TournamentRules{}, err
		}

		if !normalized.accountingEnabled() {
			return TournamentRules{}, errors.New("percent payouts require a buy-in or a guarantee")
		}
	case PayoutModeWinnerTakeAll:
		if len(normalized.Payouts) > 0 {
			return TournamentRules{}, errors.New("fixed payouts are not allowed in winner-take-all mode")
		}

		if len(normalized.PercentPayouts) > 0 {
			return TournamentRules{}, errors.New("percent payouts are not allowed in winner-take-all mode")
		}

		if !normalized.accountingEnabled() {
			return TournamentRules{}, errors.New("winner-take-all mode requires a buy-in or a guarantee")
		}
	default:
		return TournamentRules{}, fmt.Errorf("unknown payout mode %q", normalized.PayoutMode)
	}

	return normalized, nil
}

// validateFixedPayouts validates and sorts fixed payouts by place.
func validateFixedPayouts(rules *TournamentRules) error {
	if len(rules.Payouts) == 0 {
		return errors.New("at least one payout is required")
	}

	sort.Slice(rules.Payouts, func(i, j int) bool {
		return rules.Payouts[i].Place < rules.Payouts[j].Place
	})

	seen := make(map[int]bool, len(rules.Payouts))

	for i, payout := range rules.Payouts {
		if payout.Place <= 0 {
			return fmt.Errorf("payout place must be positive, got %d", payout.Place)
		}

		if payout.Amount <= 0 {
			return fmt.Errorf("payout amount for place %d must be positive, got %d", payout.Place, payout.Amount)
		}

		if seen[payout.Place] {
			return fmt.Errorf("duplicate payout place %d", payout.Place)
		}

		seen[payout.Place] = true

		if payout.Place != i+1 {
			return fmt.Errorf("payout places must be contiguous starting at 1, got %d at index %d", payout.Place, i)
		}
	}

	if len(rules.Payouts) > rules.MaxPlayers {
		return fmt.Errorf(
			"%d payout places exceed the maximum of %d players",
			len(rules.Payouts),
			rules.MaxPlayers,
		)
	}

	return nil
}

// validatePercentPayouts validates and sorts percent payouts by place.
func validatePercentPayouts(rules *TournamentRules) error {
	if len(rules.PercentPayouts) == 0 {
		return errors.New("at least one percent payout is required")
	}

	sort.Slice(rules.PercentPayouts, func(i, j int) bool {
		return rules.PercentPayouts[i].Place < rules.PercentPayouts[j].Place
	})

	seen := make(map[int]bool, len(rules.PercentPayouts))
	total := 0

	for i, payout := range rules.PercentPayouts {
		if payout.Place <= 0 {
			return fmt.Errorf("percent payout place must be positive, got %d", payout.Place)
		}

		if payout.Percent <= 0 {
			return fmt.Errorf("percent payout for place %d must be positive, got %d", payout.Place, payout.Percent)
		}

		if payout.Percent > 100 {
			return fmt.Errorf("percent payout for place %d must not exceed 100, got %d", payout.Place, payout.Percent)
		}

		if seen[payout.Place] {
			return fmt.Errorf("duplicate percent payout place %d", payout.Place)
		}

		seen[payout.Place] = true

		if payout.Place != i+1 {
			return fmt.Errorf("percent payout places must be contiguous starting at 1, got %d at index %d", payout.Place, i)
		}

		total += payout.Percent
	}

	if total != 100 {
		return fmt.Errorf("percent payouts must sum to 100, got %d", total)
	}

	if len(rules.PercentPayouts) > rules.MaxPlayers {
		return fmt.Errorf(
			"%d percent payout places exceed the maximum of %d players",
			len(rules.PercentPayouts),
			rules.MaxPlayers,
		)
	}

	return nil
}

// payoutPlaces returns the number of places that receive a prize.
func (r TournamentRules) payoutPlaces() int {
	switch r.PayoutMode {
	case PayoutModePercent:
		return len(r.PercentPayouts)
	case PayoutModeWinnerTakeAll:
		return 1
	default:
		return len(r.Payouts)
	}
}

// accountingEnabled reports whether a buy-in or guarantee funds the pool.
func (r TournamentRules) accountingEnabled() bool {
	return r.BuyIn > 0 || r.Guarantee > 0
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
	rules.PercentPayouts = append([]PercentPayout(nil), rules.PercentPayouts...)

	return rules
}

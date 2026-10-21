package models

import (
	"errors"
	"fmt"
)

// HandPhase is the lifecycle phase of a hand.
type HandPhase string

const (
	// PhaseWaiting means no hand has been started yet.
	PhaseWaiting HandPhase = "waiting"

	// PhaseBetting means a betting round is in progress.
	PhaseBetting HandPhase = "betting"

	// PhaseStreetComplete means the current betting round is complete and
	// NextStreet may be called.
	PhaseStreetComplete HandPhase = "street-complete"

	// PhaseShowdown means betting is over and Showdown may be called.
	PhaseShowdown HandPhase = "showdown"

	// PhaseComplete means the hand has been paid out.
	PhaseComplete HandPhase = "complete"
)

// Street identifies the current betting street.
type Street string

const (
	StreetPreFlop  Street = "preflop"
	StreetFlop     Street = "flop"
	StreetTurn     Street = "turn"
	StreetRiver    Street = "river"
	StreetShowdown Street = "showdown"
)

// ActionType is a legal poker action.
type ActionType string

const (
	ActionCheck ActionType = "check"
	ActionCall  ActionType = "call"
	ActionBet   ActionType = "bet"
	ActionRaise ActionType = "raise"
	ActionFold  ActionType = "fold"
)

// Action is a player action. For Bet and Raise, Amount is the player's
// total commitment on the current street ("raise to"), not the increment.
type Action struct {
	PlayerID string
	Type     ActionType
	Amount   Chips
}

// ActionResult describes what happened after a successful action.
type ActionResult struct {
	PlayerID    string
	Type        ActionType
	Paid        Chips
	PotTotal    Chips
	Street      Street
	Phase       HandPhase
	NextActorID string
}

// Phase returns the current hand phase.
func (t *Table) Phase() HandPhase {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.phase
}

// Street returns the current betting street.
func (t *Table) Street() Street {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.street
}

// CurrentBet returns the highest street commitment.
func (t *Table) CurrentBet() Chips {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.currentBet
}

// MinRaise returns the minimum raise increment for the current street.
func (t *Table) MinRaise() Chips {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.lastFullRaise
}

// NeedsNextStreet reports whether NextStreet should be called.
func (t *Table) NeedsNextStreet() bool {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.phase == PhaseStreetComplete
}

// CurrentActor returns the player whose turn it is.
func (t *Table) CurrentActor() (*Player, error) {
	t.m.RLock()
	defer t.m.RUnlock()

	if t.phase != PhaseBetting {
		return nil, fmt.Errorf("no player is acting in phase %s", t.phase)
	}

	if t.currentActor < 0 || t.currentActor >= len(t.Players) {
		return nil, errors.New("no current actor")
	}

	return t.Players[t.currentActor], nil
}

// ToCall returns how many chips the player must add to match the current
// bet.
func (t *Table) ToCall(playerID string) (Chips, error) {
	t.m.RLock()
	defer t.m.RUnlock()

	p := t.getPlayerByID(playerID)
	if p == nil {
		return 0, fmt.Errorf("player with ID %s is not at this table", playerID)
	}

	if !p.Active {
		return 0, fmt.Errorf("player %s is not in the hand", playerID)
	}

	toCall := t.currentBet - p.GetStreetBet()
	if toCall < 0 {
		toCall = 0
	}

	return toCall, nil
}

// LegalActions returns the action types currently available to the player.
func (t *Table) LegalActions(playerID string) ([]ActionType, error) {
	t.m.RLock()
	defer t.m.RUnlock()

	if t.phase != PhaseBetting {
		return nil, fmt.Errorf("no betting in phase %s", t.phase)
	}

	p := t.getPlayerByID(playerID)
	if p == nil {
		return nil, fmt.Errorf("player with ID %s is not at this table", playerID)
	}

	if !t.canActLocked(p) {
		return nil, fmt.Errorf("player %s cannot act now", playerID)
	}

	if t.currentActor < 0 || t.Players[t.currentActor].ID != playerID {
		return nil, fmt.Errorf("it is not player %s's turn", playerID)
	}

	toCall := t.currentBet - p.GetStreetBet()
	if toCall < 0 {
		toCall = 0
	}

	actions := make([]ActionType, 0, 4)

	if toCall == 0 {
		actions = append(actions, ActionCheck)

		if t.canRaiseLocked(p) {
			if t.currentBet == 0 {
				actions = append(actions, ActionBet)
			} else {
				actions = append(actions, ActionRaise)
			}
		}
	} else {
		actions = append(actions, ActionCall)

		if t.currentBet > 0 && t.canRaiseLocked(p) && p.GetCurrentChipsAmount() > toCall {
			actions = append(actions, ActionRaise)
		}
	}

	actions = append(actions, ActionFold)

	return actions, nil
}

// Act performs a player action and advances the turn.
func (t *Table) Act(action Action) (ActionResult, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.phase != PhaseBetting {
		return ActionResult{}, fmt.Errorf("cannot act in phase %s", t.phase)
	}

	p := t.getPlayerByID(action.PlayerID)
	if p == nil {
		return ActionResult{}, fmt.Errorf("player with ID %s is not at this table", action.PlayerID)
	}

	if t.currentActor < 0 || t.currentActor >= len(t.Players) || t.Players[t.currentActor].ID != p.ID {
		return ActionResult{}, fmt.Errorf("it is not player %s's turn", p.ID)
	}

	if !t.canActLocked(p) {
		return ActionResult{}, fmt.Errorf("player %s cannot act now", p.ID)
	}

	toCall := t.currentBet - p.GetStreetBet()
	if toCall < 0 {
		toCall = 0
	}

	var paid Chips

	switch action.Type {
	case ActionCheck:
		if toCall != 0 {
			return ActionResult{}, fmt.Errorf("cannot check facing a bet of %d", toCall)
		}

		p.markActed()

	case ActionCall:
		if toCall <= 0 {
			return ActionResult{}, errors.New("nothing to call, use check")
		}

		amount := toCall
		if stack := p.GetCurrentChipsAmount(); stack < amount {
			amount = stack
		}

		if amount <= 0 {
			return ActionResult{}, errors.New("player has no chips to call with")
		}

		var err error
		paid, err = p.commit(amount)
		if err != nil {
			return ActionResult{}, err
		}

		if _, err := t.Pot.AddPlayerBet(paid, p.ID); err != nil {
			return ActionResult{}, err
		}

		p.markActed()

	case ActionBet:
		if t.currentBet != 0 {
			return ActionResult{}, errors.New("cannot bet when a bet already exists, use raise")
		}

		if !t.canRaiseLocked(p) {
			return ActionResult{}, errors.New("raising is not allowed")
		}

		target := action.Amount
		streetBet := p.GetStreetBet()
		maxTarget := streetBet + p.GetCurrentChipsAmount()

		if target <= streetBet {
			return ActionResult{}, errors.New("bet amount must be positive")
		}

		if target > maxTarget {
			return ActionResult{}, fmt.Errorf("bet amount %d exceeds stack %d", target, maxTarget-streetBet)
		}

		isAllIn := target == maxTarget
		if !isAllIn && target < t.rules.MinRaise {
			return ActionResult{}, fmt.Errorf("minimum bet is %d, got %d", t.rules.MinRaise, target)
		}

		var err error
		paid, err = p.commit(target - streetBet)
		if err != nil {
			return ActionResult{}, err
		}

		if _, err := t.Pot.AddPlayerBet(paid, p.ID); err != nil {
			return ActionResult{}, err
		}

		t.currentBet = target
		t.raisesThisStreet++

		if target >= t.rules.MinRaise {
			t.lastFullRaise = paid
			t.lastFullRaiseBet = target
			t.resetActedExceptLocked(p)
		}

		p.markActed()

	case ActionRaise:
		if t.currentBet == 0 {
			return ActionResult{}, errors.New("cannot raise when there is no bet, use bet")
		}

		if !t.canRaiseLocked(p) {
			return ActionResult{}, errors.New("raising is not allowed")
		}

		target := action.Amount
		streetBet := p.GetStreetBet()
		maxTarget := streetBet + p.GetCurrentChipsAmount()

		if target <= t.currentBet {
			return ActionResult{}, fmt.Errorf("raise must be greater than current bet %d", t.currentBet)
		}

		if target > maxTarget {
			return ActionResult{}, fmt.Errorf("raise amount %d exceeds stack %d", target, maxTarget-streetBet)
		}

		increment := target - t.currentBet
		isAllIn := target == maxTarget
		fullRaise := increment >= t.lastFullRaise

		if !isAllIn && !fullRaise {
			return ActionResult{}, fmt.Errorf("minimum raise is %d, got %d", t.lastFullRaise, increment)
		}

		var err error
		paid, err = p.commit(target - streetBet)
		if err != nil {
			return ActionResult{}, err
		}

		if _, err := t.Pot.AddPlayerBet(paid, p.ID); err != nil {
			return ActionResult{}, err
		}

		t.currentBet = target
		t.raisesThisStreet++

		if fullRaise {
			t.lastFullRaise = increment
			t.lastFullRaiseBet = target
			t.resetActedExceptLocked(p)
		}

		p.markActed()

	case ActionFold:
		p.fold()

	default:
		return ActionResult{}, fmt.Errorf("unknown action type %q", action.Type)
	}

	t.advanceAfterActionLocked()

	result := ActionResult{
		PlayerID: p.ID,
		Type:     action.Type,
		Paid:     paid,
		PotTotal: t.Pot.TotalChipsNum,
		Street:   t.street,
		Phase:    t.phase,
	}

	if t.currentActor >= 0 && t.currentActor < len(t.Players) {
		result.NextActorID = t.Players[t.currentActor].ID
	}

	return result, nil
}

func (t *Table) canActLocked(p *Player) bool {
	return p.Active && !p.IsAllIn() && p.GetCurrentChipsAmount() > 0
}

func (t *Table) canRaiseLocked(p *Player) bool {
	if t.rules.MaxRaisesPerStreet != 0 && t.raisesThisStreet >= t.rules.MaxRaisesPerStreet {
		return false
	}

	// A short all-in raise does not reopen betting for players who have
	// already acted on this street.
	if p.HasActed() && t.currentBet > t.lastFullRaiseBet {
		return false
	}

	return true
}

func (t *Table) resetActedExceptLocked(actor *Player) {
	for _, p := range t.Players {
		if p.ID == actor.ID || !p.Active || p.IsAllIn() {
			continue
		}

		p.mu.Lock()
		p.acted = false
		p.mu.Unlock()
	}
}

func (t *Table) roundCompleteLocked() bool {
	if t.activeCountLocked() <= 1 {
		return true
	}

	for _, p := range t.Players {
		if !p.Active || p.IsAllIn() || p.GetCurrentChipsAmount() == 0 {
			continue
		}

		if !p.HasActed() || p.GetStreetBet() != t.currentBet {
			return false
		}
	}

	return true
}

func (t *Table) nextToActIndexLocked(start int) int {
	n := len(t.Players)
	if n == 0 {
		return -1
	}

	for i := 0; i < n; i++ {
		idx := ((start % n) + i + n) % n
		p := t.Players[idx]

		if !p.Active || p.IsAllIn() || p.GetCurrentChipsAmount() == 0 {
			continue
		}

		if p.HasActed() && p.GetStreetBet() == t.currentBet {
			continue
		}

		return idx
	}

	return -1
}

func (t *Table) advanceAfterActionLocked() {
	if t.activeCountLocked() <= 1 {
		t.phase = PhaseShowdown
		t.street = StreetShowdown
		t.currentActor = -1

		return
	}

	if t.roundCompleteLocked() {
		t.phase = PhaseStreetComplete
		t.currentActor = -1

		return
	}

	next := t.nextToActIndexLocked(t.currentActor + 1)
	if next < 0 {
		t.phase = PhaseStreetComplete
		t.currentActor = -1

		return
	}

	t.currentActor = next
}

func (t *Table) firstPreFlopActorLocked(bigBlindIdx int) int {
	start := bigBlindIdx + 1
	if t.activeCountLocked() == 2 {
		start = t.Dealer
	}

	return t.nextToActIndexLocked(start)
}

func (t *Table) firstPostFlopActorLocked() int {
	return t.nextToActIndexLocked(t.Dealer + 1)
}

func (t *Table) dealPocketsLocked() error {
	if t.deck.Len() == 0 {
		return errors.New("deck is not shuffled")
	}

	for round := 0; round < PocketSize; round++ {
		for _, p := range t.Players {
			if !p.Active {
				continue
			}

			card, err := t.deck.Card()
			if err != nil {
				return err
			}

			if err := p.AddCard(card); err != nil {
				return err
			}
		}
	}

	return nil
}

func (t *Table) resetStreetStateLocked() {
	for _, p := range t.Players {
		p.resetForStreet()
	}

	t.currentBet = 0
	t.lastFullRaise = t.rules.MinRaise
	t.lastFullRaiseBet = 0
	t.raisesThisStreet = 0
	t.currentActor = -1
}

// NextStreet opens the next street when the current betting round is
// complete. If every remaining player is all-in, it runs the board out to
// showdown automatically.
func (t *Table) NextStreet() (*Table, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.phase != PhaseStreetComplete {
		return nil, fmt.Errorf("cannot open the next street in phase %s", t.phase)
	}

	if t.activeCountLocked() <= 1 {
		t.phase = PhaseShowdown
		t.street = StreetShowdown
		t.currentActor = -1

		return t, nil
	}

	if t.street == StreetRiver {
		t.phase = PhaseShowdown
		t.street = StreetShowdown
		t.currentActor = -1

		return t, nil
	}

	t.resetStreetStateLocked()

	if err := t.dealNextStreetLocked(); err != nil {
		return nil, err
	}

	if t.firstPostFlopActorLocked() < 0 {
		for t.street != StreetRiver {
			if err := t.dealNextStreetLocked(); err != nil {
				return nil, err
			}
		}

		t.street = StreetShowdown
		t.phase = PhaseShowdown
		t.currentActor = -1

		return t, nil
	}

	t.phase = PhaseBetting
	t.currentActor = t.firstPostFlopActorLocked()

	return t, nil
}

func (t *Table) dealNextStreetLocked() error {
	switch t.street {
	case StreetPreFlop:
		if _, err := t.openBoardCards(CardsOnFlopNumber); err != nil {
			return err
		}

		t.street = StreetFlop

	case StreetFlop:
		if _, err := t.openOne(3); err != nil {
			return err
		}

		t.street = StreetTurn

	case StreetTurn:
		if _, err := t.openOne(4); err != nil {
			return err
		}

		t.street = StreetRiver

	case StreetRiver:
		t.street = StreetShowdown

	default:
		return fmt.Errorf("unknown street %q", t.street)
	}

	return nil
}

// LastResults returns the payouts from the most recent showdown.
func (t *Table) LastResults() []PotResult {
	t.m.RLock()
	defer t.m.RUnlock()

	return clonePotResults(t.lastResults)
}

package engine

import (
	"errors"
	"fmt"
)

// betweenHandsLocked reports whether no hand is currently in progress, so
// the table seats and stacks may be changed. Requires t.m to be held.
func (t *Table) betweenHandsLocked() bool {
	return t.phase == PhaseWaiting || t.phase == PhaseComplete
}

// requireBetweenHandsLocked returns an error when a lifecycle operation is
// attempted during an active hand or showdown. Requires t.m to be held.
func (t *Table) requireBetweenHandsLocked(operation string) error {
	if !t.betweenHandsLocked() {
		return fmt.Errorf("cannot %s in phase %s", operation, t.phase)
	}

	return nil
}

// ReplacePlayer puts player into the seat currently held by oldID. The old
// player must be busted (0 chips), so this is used to re-enter an
// eliminated seat or to swap an eliminated player. The seat index does not
// change, so the dealer button stays valid. The pot is untouched; the next
// StartHand resets and re-registers the players.
func (t *Table) ReplacePlayer(oldID string, player *Player) error {
	if player == nil {
		return errors.New("player is nil")
	}

	t.m.Lock()
	defer t.m.Unlock()

	if err := t.requireBetweenHandsLocked("replace a player"); err != nil {
		return err
	}

	return t.replacePlayerLocked(oldID, player)
}

// replacePlayerLocked validates and performs the seat replacement. Requires
// t.m to be held.
func (t *Table) replacePlayerLocked(oldID string, player *Player) error {
	if player.ID == "" {
		return errors.New("player ID is empty")
	}

	if player.ID == oldID {
		return fmt.Errorf("new player ID %s must differ from the replaced player ID", player.ID)
	}

	if t.getPlayerByID(player.ID) != nil {
		return fmt.Errorf("player with ID %s is in this game already", player.ID)
	}

	idx := -1

	for i, p := range t.Players {
		if p.ID == oldID {
			idx = i

			break
		}
	}

	if idx < 0 {
		return fmt.Errorf("player with ID %s is not at this table", oldID)
	}

	if chips := t.Players[idx].GetCurrentChipsAmount(); chips != 0 {
		return fmt.Errorf("player with ID %s still has %d chips", oldID, chips)
	}

	if player.GetCurrentChipsAmount() <= 0 {
		return fmt.Errorf("player %s must have a positive stack", player.ID)
	}

	// Reset before seating so per-hand leftovers (cards, bets, flags) from
	// the new player's previous table cannot leak into the next hand.
	player.ResetForHand()

	t.Players[idx] = player

	return nil
}

// SeatPlayer seats a late registrant between hands. When the table has a
// free seat the player is appended; on a full table the first busted seat
// in seat order is reused. It returns an error when every seat is held by
// a funded player.
func (t *Table) SeatPlayer(player *Player) error {
	if player == nil {
		return errors.New("player is nil")
	}

	t.m.Lock()
	defer t.m.Unlock()

	if err := t.requireBetweenHandsLocked("seat a player"); err != nil {
		return err
	}

	if player.ID == "" {
		return errors.New("player ID is empty")
	}

	if t.getPlayerByID(player.ID) != nil {
		return fmt.Errorf("player with ID %s is in this game already", player.ID)
	}

	if player.GetCurrentChipsAmount() <= 0 {
		return fmt.Errorf("player %s must have a positive stack", player.ID)
	}

	if t.MaxPlayersNum <= 0 || len(t.Players) < t.MaxPlayersNum {
		player.ResetForHand()

		// Register in the pot before appending so a pot error leaves the
		// table unchanged, matching Register.
		if err := t.Pot.Register(player.ID); err != nil {
			return err
		}

		t.Players = append(t.Players, player)

		return nil
	}

	for _, p := range t.Players {
		if p.GetCurrentChipsAmount() == 0 {
			return t.replacePlayerLocked(p.ID, player)
		}
	}

	return fmt.Errorf("table %s has no free or busted seats", t.Name)
}

// AddChips adds chips to a seated player between hands, for example a
// tournament add-on. A player who is not sitting out becomes active when
// the stack is positive.
func (t *Table) AddChips(playerID string, amount Chips) error {
	t.m.Lock()
	defer t.m.Unlock()

	if err := t.requireBetweenHandsLocked("add chips"); err != nil {
		return err
	}

	if amount <= 0 {
		return fmt.Errorf("chip amount must be positive, got %d", amount)
	}

	p := t.getPlayerByID(playerID)
	if p == nil {
		return fmt.Errorf("player with ID %s is not at this table", playerID)
	}

	return p.addChipsForLifecycle(amount)
}

// SitOut marks a seated player as sitting out between hands. The player is
// deactivated and will not be dealt into the next hand, post blinds or
// antes, or hold the dealer button. Calling it for a player who is already
// sitting out is a no-op.
func (t *Table) SitOut(playerID string) error {
	t.m.Lock()
	defer t.m.Unlock()

	if err := t.requireBetweenHandsLocked("sit a player out"); err != nil {
		return err
	}

	p := t.getPlayerByID(playerID)
	if p == nil {
		return fmt.Errorf("player with ID %s is not at this table", playerID)
	}

	p.setSittingOut(true)

	return nil
}

// SitIn clears a player's sit-out status between hands. The player must
// have chips and becomes active for the next hand.
func (t *Table) SitIn(playerID string) error {
	t.m.Lock()
	defer t.m.Unlock()

	if err := t.requireBetweenHandsLocked("sit a player in"); err != nil {
		return err
	}

	p := t.getPlayerByID(playerID)
	if p == nil {
		return fmt.Errorf("player with ID %s is not at this table", playerID)
	}

	if p.GetCurrentChipsAmount() <= 0 {
		return fmt.Errorf("player %s has no chips to sit in with", playerID)
	}

	p.setSittingOut(false)

	return nil
}

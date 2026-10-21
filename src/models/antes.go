package models

import "fmt"

// blindIndexesLocked returns the seat indexes of the small and big blind.
// Heads-up the dealer posts the small blind; otherwise the small blind is
// the next active seat after the dealer and the big blind is the next
// active seat after the small blind. Requires t.m to be held.
func (t *Table) blindIndexesLocked() (int, int, error) {
	activeNum := t.activeCountLocked()
	if activeNum < 2 {
		return 0, 0, fmt.Errorf("blinds require at least 2 active players, got %d", activeNum)
	}

	n := len(t.Players)
	dealer := ((t.Dealer % n) + n) % n

	var (
		smallIdx int
		err      error
	)

	if activeNum == 2 {
		// Heads-up: the dealer posts the small blind.
		smallIdx = dealer
		if !t.Players[smallIdx].Active {
			smallIdx, err = t.nextActiveIndex(dealer + 1)
			if err != nil {
				return 0, 0, err
			}
		}
	} else {
		smallIdx, err = t.nextActiveIndex(dealer + 1)
		if err != nil {
			return 0, 0, err
		}
	}

	bigIdx, err := t.nextActiveIndex(smallIdx + 1)
	if err != nil {
		return 0, 0, err
	}

	return smallIdx, bigIdx, nil
}

// postAntesLocked posts the configured ante. In per-player mode every
// active player posts, clockwise from the dealer; in big-blind mode only
// the big blind posts. Requires t.m to be held.
func (t *Table) postAntesLocked(bigBlindIdx int) error {
	if t.rules.Ante <= 0 {
		return nil
	}

	switch t.rules.AnteMode {
	case "", AntePerPlayer:
		n := len(t.Players)

		for i := 1; i <= n; i++ {
			idx := ((t.Dealer+i)%n + n) % n
			p := t.Players[idx]

			if !p.Active {
				continue
			}

			if err := t.postAnteLocked(p); err != nil {
				return err
			}
		}

	case AnteBigBlind:
		if bigBlindIdx < 0 || bigBlindIdx >= len(t.Players) {
			return fmt.Errorf("big blind index %d is out of range", bigBlindIdx)
		}

		return t.postAnteLocked(t.Players[bigBlindIdx])

	default:
		return fmt.Errorf("unknown ante mode %q", t.rules.AnteMode)
	}

	return nil
}

// postAnteLocked deducts min(ante, stack) as dead money and records it in
// the pot. The street bet and the current bet are not affected. Requires
// t.m to be held.
func (t *Table) postAnteLocked(p *Player) error {
	amount := t.rules.Ante
	if stack := p.GetCurrentChipsAmount(); stack < amount {
		amount = stack
	}

	if amount <= 0 {
		return nil
	}

	paid, err := p.commitDead(amount)
	if err != nil {
		return err
	}

	if _, err := t.Pot.AddPlayerBet(paid, p.ID); err != nil {
		return err
	}

	return nil
}

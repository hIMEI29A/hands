package engine

import (
	"errors"
	"fmt"
	"sort"
)

// SidePot is one layer of the pot. Amount is the number of chips in the
// layer and Eligible lists the players who may win it.
type SidePot struct {
	Amount   Chips
	Eligible []string
}

// PotResult describes one payout or refund produced at showdown.
type PotResult struct {
	Amount   Chips
	Eligible []string
	Winners  []string
	Payouts  map[string]Chips
	Refunded bool
}

// Showdown evaluates the remaining hands, builds the main and side pots,
// pays the winners and marks the hand complete.
func (t *Table) Showdown() ([]PotResult, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.phase != PhaseShowdown {
		return nil, fmt.Errorf("cannot resolve showdown in phase %s", t.phase)
	}

	results, err := t.resolvePotsLocked(true)
	if err != nil {
		return nil, err
	}

	t.lastResults = clonePotResults(results)
	t.phase = PhaseComplete
	t.street = StreetShowdown
	t.currentActor = -1

	return results, nil
}

// ResolvePots calculates the main and side pots and their payouts without
// changing any stacks or the pot.
func (t *Table) ResolvePots() ([]PotResult, error) {
	t.m.RLock()
	defer t.m.RUnlock()

	if t.phase == PhaseComplete {
		return clonePotResults(t.lastResults), nil
	}

	if t.phase != PhaseShowdown {
		return nil, fmt.Errorf("cannot resolve pots in phase %s", t.phase)
	}

	return t.resolvePotsLocked(false)
}

// resolvePotsLocked builds payouts for every pot layer. When award is
// true, chips are moved from the pot to the players. Requires t.m to be
// held (read or write).
func (t *Table) resolvePotsLocked(award bool) ([]PotResult, error) {
	active := make(map[string]bool, len(t.Players))
	activeIDs := make([]string, 0, len(t.Players))

	for _, p := range t.Players {
		if p.Active {
			active[p.ID] = true
			activeIDs = append(activeIDs, p.ID)
		}
	}

	if len(activeIDs) == 0 {
		return nil, errors.New("no active players to resolve")
	}

	if len(activeIDs) > 1 && len(t.Board) != BoardSize {
		return nil, fmt.Errorf("board must contain exactly %d cards to resolve a showdown, got %d", BoardSize, len(t.Board))
	}

	contrib := make(map[string]Chips, len(t.Pot.PlayersChips))
	for id, amount := range t.Pot.PlayersChips {
		contrib[id] = amount
	}

	results := make([]PotResult, 0, len(contrib))

	if id, amount := uncalledRefund(contrib); amount > 0 {
		if t.Pot.TotalChipsNum < amount {
			return nil, fmt.Errorf("refund %d exceeds pot total %d", amount, t.Pot.TotalChipsNum)
		}

		results = append(results, PotResult{
			Amount:   amount,
			Eligible: []string{id},
			Winners:  []string{id},
			Payouts:  map[string]Chips{id: amount},
			Refunded: true,
		})

		contrib[id] -= amount
	}

	if len(activeIDs) == 1 {
		remaining := Chips(0)
		for _, amount := range contrib {
			remaining += amount
		}

		if remaining > 0 {
			results = append(results, PotResult{
				Amount:   remaining,
				Eligible: []string{activeIDs[0]},
				Winners:  []string{activeIDs[0]},
				Payouts:  map[string]Chips{activeIDs[0]: remaining},
			})
		}
	} else {
		pots, err := buildSidePots(contrib, active)
		if err != nil {
			return nil, err
		}

		board := NewStringSliceFromCards(t.Board)
		hands := make(map[string]*Hand, len(activeIDs))
		seatOrder := t.seatOrderLocked()

		for _, pot := range pots {
			winners, err := t.potWinnersLocked(pot.Eligible, board, hands)
			if err != nil {
				return nil, err
			}

			results = append(results, PotResult{
				Amount:   pot.Amount,
				Eligible: pot.Eligible,
				Winners:  winners,
				Payouts:  splitPayouts(pot.Amount, winners, seatOrder),
			})
		}
	}

	if award {
		if err := t.applyResultsLocked(results); err != nil {
			return nil, err
		}
	}

	return results, nil
}

func (t *Table) potWinnersLocked(eligible []string, board []string, hands map[string]*Hand) ([]string, error) {
	if len(eligible) == 0 {
		return nil, errors.New("pot has no eligible players")
	}

	var best *Hand

	for _, id := range eligible {
		h, ok := hands[id]
		if !ok {
			p := t.getPlayerByID(id)
			if p == nil {
				return nil, fmt.Errorf("player with ID %s is not at this table", id)
			}

			pocket := p.GetPocketCards()
			if len(pocket) != PocketSize {
				return nil, fmt.Errorf("player %s has %d pocket cards, want %d", p.ID, len(pocket), PocketSize)
			}

			var err error
			h, err = GetMaxHandWithBoard(board, NewStringSliceFromCards(pocket))
			if err != nil {
				return nil, err
			}

			hands[id] = h
		}

		if best == nil || h.Compare(best) > 0 {
			best = h
		}
	}

	winners := make([]string, 0, len(eligible))
	for _, id := range eligible {
		if hands[id].Compare(best) == 0 {
			winners = append(winners, id)
		}
	}

	sort.Strings(winners)

	return winners, nil
}

// seatOrderLocked returns player IDs clockwise from the dealer.
func (t *Table) seatOrderLocked() map[string]int {
	order := make(map[string]int, len(t.Players))
	n := len(t.Players)

	for i := 0; i < n; i++ {
		idx := (t.Dealer + 1 + i) % n
		order[t.Players[idx].ID] = i
	}

	return order
}

func splitPayouts(amount Chips, winners []string, seatOrder map[string]int) map[string]Chips {
	payouts := make(map[string]Chips, len(winners))
	if len(winners) == 0 {
		return payouts
	}

	base := amount / Chips(len(winners))
	remainder := int(amount % Chips(len(winners)))

	for _, id := range winners {
		payouts[id] = base
	}

	ordered := make([]string, len(winners))
	copy(ordered, winners)

	sort.Slice(ordered, func(i, j int) bool {
		return seatOrder[ordered[i]] < seatOrder[ordered[j]]
	})

	for i := 0; i < remainder && i < len(ordered); i++ {
		payouts[ordered[i]]++
	}

	return payouts
}

func (t *Table) applyResultsLocked(results []PotResult) error {
	var total Chips

	for _, result := range results {
		total += result.Amount
	}

	if total != t.Pot.TotalChipsNum {
		return fmt.Errorf("payout total %d does not match pot total %d", total, t.Pot.TotalChipsNum)
	}

	for _, result := range results {
		for id, amount := range result.Payouts {
			p := t.getPlayerByID(id)
			if p == nil {
				return fmt.Errorf("player with ID %s is not at this table", id)
			}

			if err := p.addChips(amount); err != nil {
				return err
			}

			if err := t.Pot.RemoveChips(amount); err != nil {
				return err
			}
		}
	}

	return nil
}

// uncalledRefund finds the part of the largest contribution that no other
// player matched. It returns the contributor ID and the refund amount.
func uncalledRefund(contrib map[string]Chips) (string, Chips) {
	var (
		maxID   string
		max     Chips
		maxTies int
	)

	for id, amount := range contrib {
		switch {
		case amount > max:
			max = amount
			maxID = id
			maxTies = 1
		case amount == max && amount > 0:
			maxTies++
		}
	}

	if max == 0 || maxTies != 1 {
		return "", 0
	}

	var second Chips

	for id, amount := range contrib {
		if id == maxID {
			continue
		}

		if amount > second {
			second = amount
		}
	}

	if max <= second {
		return "", 0
	}

	return maxID, max - second
}

// buildSidePots splits contributions into layers. Folded players
// contribute chips but are not eligible to win.
func buildSidePots(contrib map[string]Chips, active map[string]bool) ([]SidePot, error) {
	levels := make([]Chips, 0, len(contrib))
	seen := make(map[Chips]bool, len(contrib))

	for _, amount := range contrib {
		if amount > 0 && !seen[amount] {
			seen[amount] = true
			levels = append(levels, amount)
		}
	}

	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })

	pots := make([]SidePot, 0, len(levels))
	var previous Chips

	for _, level := range levels {
		amount := level - previous
		participants := 0
		eligible := make([]string, 0, len(contrib))

		for id, contribution := range contrib {
			if contribution < level {
				continue
			}

			participants++

			if active[id] {
				eligible = append(eligible, id)
			}
		}

		potAmount := amount * Chips(participants)
		if potAmount > 0 {
			if len(eligible) == 0 {
				return nil, fmt.Errorf("side pot at level %d has no eligible players", level)
			}

			sort.Strings(eligible)

			// Folded contributions can create adjacent layers with the
			// same eligibility; merge them into one logical pot.
			if len(pots) > 0 && samePlayers(pots[len(pots)-1].Eligible, eligible) {
				pots[len(pots)-1].Amount += potAmount
			} else {
				pots = append(pots, SidePot{Amount: potAmount, Eligible: eligible})
			}
		}

		previous = level
	}

	return pots, nil
}

func samePlayers(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func clonePotResults(results []PotResult) []PotResult {
	out := make([]PotResult, len(results))

	for i, result := range results {
		out[i] = PotResult{
			Amount:   result.Amount,
			Eligible: append([]string(nil), result.Eligible...),
			Winners:  append([]string(nil), result.Winners...),
			Payouts:  make(map[string]Chips, len(result.Payouts)),
			Refunded: result.Refunded,
		}

		for id, amount := range result.Payouts {
			out[i].Payouts[id] = amount
		}
	}

	return out
}

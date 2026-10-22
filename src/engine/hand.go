package engine

import (
	"errors"
	"fmt"
	"sort"

	"hands/src/helpers"
)

const (
	// HandSize is the number of cards in a poker combination.
	HandSize = 5

	// PocketSize is the number of cards a player holds.
	PocketSize = 2
)

// HandValue is the poker category (strength) of a hand.
type HandValue int

const (
	HighCardHand      HandValue = iota // High card
	PairHand                           // Pair
	TwoPairHand                        // Two pairs
	ThreeHand                          // Three of a kind
	StraightHand                       // Straight
	FlushHand                          // Flush
	FullHouseHand                      // Full house
	FourHand                           // Four of a kind
	StraightFlushHand                  // Straight flush
	RoyalFlushHand                     // Royal flush
)

/* getMaxHand chooses the strongest 5-card combination out of 7 cards. */

// getMaxHand evaluates every HandSize-card combination of cards and
// returns the strongest one. It returns an error if len(cards) is not
// HandSize+PocketSize or a combination cannot be built.
// cards must be valid card strings handled by NewHandFromStrings;
// combinations from valid distinct cards never fail validation.
func getMaxHand(cards []string) (*Hand, error) {
	if len(cards) != HandSize+PocketSize {
		return nil, fmt.Errorf("wrong number of cards to calculate combinations: got %d, want %d", len(cards), HandSize+PocketSize)
	}

	combos := helpers.GetCombinations(cards, HandSize)

	var best *Hand

	for _, combo := range combos {
		h, err := NewHandFromStrings(combo)
		if err != nil {
			return nil, err
		}

		if best == nil || h.Compare(best) > 0 {
			best = h
		}
	}

	return best, nil
}

/* Hand */

// Hand is a five-card poker combination. The cards are kept sorted
// ascending by value (suit as a tiebreaker), so iterating over a hand is
// deterministic. Hand cards are copies: mutating the passed slice after
// construction does not change the hand.
type Hand struct {
	cards []*Card
}

// compareCards orders two cards by value and then by suit; it returns -1,
// 0 or 1 and is used to keep a hand's cards in a deterministic order.
func compareCards(a, b *Card) int {
	if a.Value < b.Value {
		return -1
	}
	if a.Value > b.Value {
		return 1
	}

	aSuit, bSuit := a.Suit.Suit, b.Suit.Suit
	switch {
	case aSuit < bSuit:
		return -1
	case aSuit > bSuit:
		return 1
	}

	return 0
}

// countOf returns how many groups of exactly n cards of the same value
// are in the hand (e.g. countOf(2) returns the number of pairs).
func (h *Hand) countOf(n int) int {
	counts := make(map[CardValue]int)
	for _, c := range h.cards {
		counts[c.Value]++
	}

	result := 0
	for _, count := range counts {
		if count == n {
			result++
		}
	}

	return result
}

/* Hand constructors and validation. */

// ValidateHand checks that a 5-card slice can be used as a Hand: exactly
// HandSize cards, no nil elements and no duplicate value+suit pairs.
func ValidateHand(hand []*Card) error {
	if hand == nil {
		return errors.New("hand must not be nil")
	}

	if len(hand) != HandSize {
		return fmt.Errorf("hand must contain exactly %d cards, got %d", HandSize, len(hand))
	}

	seen := make(map[string]bool, len(hand))
	for i, card := range hand {
		if card == nil {
			return fmt.Errorf("hand contains a nil card at index %d", i)
		}

		if card.Suit == nil {
			return fmt.Errorf("hand contains a card with nil suit at index %d", i)
		}

		key := card.String()
		if seen[key] {
			return fmt.Errorf("hand contains duplicate card %s", key)
		}

		seen[key] = true
	}

	return nil
}

// NewHandFromCards builds a Hand from 5 valid, distinct cards. It stores a
// sorted copy (ascending by value, suit as a tiebreaker) and returns a
// descriptive error otherwise.
func NewHandFromCards(cards []*Card) (*Hand, error) {
	if err := ValidateHand(cards); err != nil {
		return nil, err
	}

	sorted := make([]*Card, HandSize)
	copy(sorted, cards)

	sort.Slice(sorted, func(i, j int) bool {
		return compareCards(sorted[i], sorted[j]) < 0
	})

	return &Hand{cards: sorted}, nil
}

// NewHandFromStrings parses card strings ("10H", "2C", "AS") with
// ParseCard and builds a Hand. The first parsing error is returned.
func NewHandFromStrings(cards []string) (*Hand, error) {
	parsed := make([]*Card, len(cards))

	for i, s := range cards {
		card, err := ParseCard(s)
		if err != nil {
			return nil, fmt.Errorf("parse card %q at index %d: %w", s, i, err)
		}

		parsed[i] = card
	}

	return NewHandFromCards(parsed)
}

/* Hand accessors and predicates. */

// Slice returns the hand's cards in sorted order (ascending by value,
// suit as a tiebreaker). The returned slice is a copy.
func (h *Hand) Slice() []*Card {
	cc := make([]*Card, len(h.cards))
	copy(cc, h.cards)

	return cc
}

// StringSlice returns the hand's cards as card strings in sorted order.
func (h *Hand) StringSlice() []string {
	ss := make([]string, len(h.cards))

	for i, c := range h.cards {
		ss[i] = c.String()
	}

	return ss
}

// Max returns the hand's highest card (nil for an empty hand).
func (h *Hand) Max() *Card {
	if len(h.cards) == 0 {
		return nil
	}

	return h.cards[len(h.cards)-1]
}

// CountPairs returns the number of pairs in the hand: a group of exactly
// two cards of the same value. Possible results for 5 cards are 0, 1 or 2.
func (h *Hand) CountPairs() int {
	return h.countOf(2)
}

// HasFlush reports whether all cards share the same suit.
func (h *Hand) HasFlush() bool {
	return isFlush(h.cards)
}

// HasStraight reports whether all cards form consecutive values. A-2-3-4-5
// (the wheel) counts as a straight with high card 5.
func (h *Hand) HasStraight() bool {
	_, ok := straightHigh(h.cards)

	return ok
}

// HasThree reports whether the hand contains three cards of one value.
func (h *Hand) HasThree() bool {
	return h.countOf(3) > 0
}

// HasFour reports whether the hand contains four cards of one value.
func (h *Hand) HasFour() bool {
	return h.countOf(4) > 0
}

// HasFullHouse reports whether the hand contains three cards of one value
// and two cards of another.
func (h *Hand) HasFullHouse() bool {
	return h.countOf(3) > 0 && h.countOf(2) > 0
}

// Same reports whether other contains exactly the same five cards (equal
// values and suits), regardless of order.
func (h *Hand) Same(other *Hand) bool {
	if h == nil || other == nil {
		return false
	}

	if len(h.cards) != len(other.cards) {
		return false
	}

	// Both hands keep their cards sorted, so position-wise comparison
	// is set equality.
	for i := range h.cards {
		if h.cards[i].String() != other.cards[i].String() {
			return false
		}
	}

	return true
}

// GetMaxHandWithBoard finds the strongest hand for a board and a player
// pocket, i.e. the best 5-card combination of the 7 cards.
func GetMaxHandWithBoard(board, pocket []string) (*Hand, error) {
	// Copy instead of append: appending to board could write into its
	// spare capacity and silently mutate the caller's slice.
	cards := make([]string, 0, len(board)+len(pocket))
	cards = append(cards, board...)
	cards = append(cards, pocket...)

	return getMaxHand(cards)
}

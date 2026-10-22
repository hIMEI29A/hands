package engine

import "sort"

/* Single poker hand evaluator. */

// valueCount is one group of cards sharing the same value: the value and
// how many cards of that value are in the hand.
type valueCount struct {
	value CardValue
	count int
}

// groupByValue counts card values in cards and returns the groups sorted
// by value descending.
func groupByValue(cards []*Card) []valueCount {
	m := make(map[CardValue]int)

	for _, c := range cards {
		m[c.Value]++
	}

	groups := make([]valueCount, 0, len(m))
	for v, n := range m {
		groups = append(groups, valueCount{value: v, count: n})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].value > groups[j].value
	})

	return groups
}

// isFlush reports whether all cards share the same suit.
func isFlush(cards []*Card) bool {
	if len(cards) == 0 {
		return false
	}

	suit := cards[0].Suit.Suit
	for _, c := range cards {
		if c.Suit.Suit != suit {
			return false
		}
	}

	return true
}

// straightHigh checks whether cards are consecutive values and, if so,
// returns the straight's high card. For exactly 5 distinct values, a span
// of 4 is necessary and sufficient for consecutive values. A-2-3-4-5 is
// additionally accepted as a straight ("wheel") with high card 5.
// Cards with duplicate values cannot form a straight.
func straightHigh(cards []*Card) (CardValue, bool) {
	groups := groupByValue(cards)
	if len(groups) != HandSize {
		// Duplicate values cannot form a straight.
		return 0, false
	}

	// groups are sorted descending, groups[0] is the highest value.
	if int(groups[0].value)-int(groups[len(groups)-1].value) == 4 {
		return groups[0].value, true
	}

	// Wheel: A-5-4-3-2 => high card 5.
	if groups[0].value == Ace && groups[1].value == Five &&
		groups[2].value == Four && groups[3].value == Three && groups[4].value == Two {
		return Five, true
	}

	return 0, false
}

// valuesDescending returns every distinct card value in cards, most
// significant first. It is meant for flush and high card hands where all
// 5 values are counted for tiebreaks.
func valuesDescending(cards []*Card) []CardValue {
	groups := groupByValue(cards)

	tb := make([]CardValue, 0, len(groups))
	for _, g := range groups {
		tb = append(tb, g.value)
	}

	return tb
}

// evaluate determines the poker category of the 5 valid, distinct cards,
// along with the ordered tiebreak values (most significant first):
//
//	HighCard:         5 values descending
//	Pairs:            pair value, then kickers descending
//	TwoPair:          high pair, low pair, kicker
//	ThreeOfKind:      trip value, then 2 kickers descending
//	Straight:         high card only (wheel => 5)
//	Flush:            5 values descending
//	FullHouse:        trip value, pair value
//	FourOfKind:       quad value, kicker
//	StraightFlush:    high card only (wheel => 5)
//	RoyalFlush:       empty tiebreak
//
// It expects exactly HandSize valid, distinct cards, as guaranteed by the
// Hand constructors; the input order does not matter.
func evaluate(cards []*Card) (HandValue, []CardValue) {
	groups := groupByValue(cards)

	flush := isFlush(cards)
	straightHi, straight := straightHigh(cards)

	switch {
	case straight && flush:
		if straightHi == Ace {
			// A-K-Q-J-10 same suit; a wheel straight flush (5-high)
			// never reaches this branch, so it stays StraightFlushHand.
			return RoyalFlushHand, nil
		}

		return StraightFlushHand, []CardValue{straightHi}
	case flush:
		return FlushHand, valuesDescending(cards)
	case straight:
		return StraightHand, []CardValue{straightHi}
	}

	// Grouped sizes are detected from value counts. Buckets keep the
	// value-descending order of groups, so each bucket's values are
	// already ordered for tiebreaks.
	var quads, trips, pairs, singles []CardValue
	for _, g := range groups {
		switch g.count {
		case 4:
			quads = append(quads, g.value)
		case 3:
			trips = append(trips, g.value)
		case 2:
			pairs = append(pairs, g.value)
		default:
			singles = append(singles, g.value)
		}
	}

	switch {
	case len(quads) == 1:
		return FourHand, []CardValue{quads[0], singles[0]}
	case len(trips) == 1 && len(pairs) == 1:
		return FullHouseHand, []CardValue{trips[0], pairs[0]}
	case len(trips) == 1:
		return ThreeHand, []CardValue{trips[0], singles[0], singles[1]}
	case len(pairs) == 2:
		return TwoPairHand, []CardValue{pairs[0], pairs[1], singles[0]}
	case len(pairs) == 1:
		return PairHand, []CardValue{pairs[0], singles[0], singles[1], singles[2]}
	default:
		return HighCardHand, singles // all five values, descending
	}
}

// compareEvaluated compares two evaluated hands: first by category, then
// by tiebreak values lexicographically. It returns -1, 0 or 1.
func compareEvaluated(catA HandValue, tbA []CardValue, catB HandValue, tbB []CardValue) int {
	if catA < catB {
		return -1
	}
	if catA > catB {
		return 1
	}

	n := len(tbA)
	if len(tbB) < n {
		n = len(tbB)
	}

	for i := 0; i < n; i++ {
		if tbA[i] < tbB[i] {
			return -1
		}
		if tbA[i] > tbB[i] {
			return 1
		}
	}

	// Same prefix: the shorter slice ranks lower. Equal lengths mean a
	// full tie.
	if len(tbA) < len(tbB) {
		return -1
	}
	if len(tbA) > len(tbB) {
		return 1
	}

	return 0
}

// Define determines the hand's poker category (HandValue). The returned
// value ignores suit and kickers; use Compare to rank two hands.
func (h *Hand) Define() HandValue {
	if h == nil || len(h.cards) != HandSize {
		return HighCardHand
	}

	category, _ := evaluate(h.cards)

	return category
}

// Compare evaluates both hands and returns -1, 0 or 1 depending on whether
// one is lower than, equal to, or greater than other. Hands of the same
// category are compared by their ordered tiebreak values, so the result is
// a total order.
func (h *Hand) Compare(other *Hand) int {
	if h == nil || other == nil {
		if h == other {
			return 0
		}
		if h == nil {
			return -1
		}

		return 1
	}

	catA, tbA := evaluate(h.cards)
	catB, tbB := evaluate(other.cards)

	return compareEvaluated(catA, tbA, catB, tbB)
}

// Compare ranks two hands with the same result as Hand.Compare.
func Compare(one, other *Hand) int {
	return one.Compare(other)
}

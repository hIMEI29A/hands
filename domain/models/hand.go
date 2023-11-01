package models

import (
	"errors"
	"hands/domain/helpers"
	"sort"
)

const HandLength = 5

type Hand int

const (
	HighCardHand Hand = iota
	PairHand
	TwoPairHand
	ThreeHand
	StraightHand
	FlushHand
	FullHouseHand
	FourHand
	StraightFlushHand
	RoyalFlushHand
)

func getMaxHand(cards []string) *HandRaw {
	combos := helpers.GetCombinations(cards, HandLength)
	hands := make([]*HandRaw, len(combos))

	for i, c := range combos {
		hands[i] = NewHandFromStrings(c)
	}

	sort.Slice(hands, func(i, j int) bool {
		return hands[i].Compare(hands[j]) < 0
	})

	return hands[len(hands)-1]
}

func max(cards []*Card) *Card {
	c := cards[0]

	for i := 1; i < len(cards); i++ {
		if cards[i].CompareValues(c) > 0 {
			c = cards[i]
		}
	}

	return c
}

func deleteCard(cards []*Card, cardsSlice ...*Card) []*Card {
	out := make([]*Card, 0)

	for _, c := range cards {
		if !inSlice(cardsSlice, c) {
			out = append(out, c)
		}
	}
	return out
}

func countDuplicates(hand []*Card) int {
	visited := make(map[string]bool)
	count := 0

	for _, card := range hand {
		if visited[card.String()] {
			count++
		} else {
			visited[card.String()] = true
		}
	}

	return count
}

func inSlice(cards []*Card, card *Card) bool {
	for _, c := range cards {
		if card.String() == c.String() {
			return true
		}
	}

	return false
}

func ValidateHand(hand []*Card) error {
	if len(hand) > 5 {
		return errors.New("Hand must contains 5 cards only")
	}

	if countDuplicates(hand) > 0 {
		return errors.New("Hand must not contains duplicates")
	}

	return nil
}

func comparePairs(first, other *HandRaw) int {
	h1, _, hc1 := first.GetPair()
	h2, _, hc2 := other.GetPair()

	c := h1[0].CompareValues(h2[0])

	if c != 0 {
		return c
	} else {
		return hc1.CompareValues(hc2)
	}
}

func compareTwoPairs(first, other *HandRaw) int {
	firsPair1, secondPair1, high1 := first.GetTwoPair()
	firsPair2, secondPair2, high2 := other.GetTwoPair()

	max1, max2 := &Card{}, &Card{}

	// находим старшую пару у первого игрока
	if firsPair1[0].CompareValues(secondPair1[0]) > 0 {
		max1 = firsPair1[0]
	} else {
		max1 = secondPair1[0]
	}

	// находим старшую пару из двух у второго игрока
	if firsPair2[0].CompareValues(secondPair2[0]) > 0 {
		max2 = firsPair2[0]
	} else {
		max2 = secondPair2[0]
	}

	// сравниваем старшие пары
	if max1.CompareValues(max2) < 0 {
		return -1
	} else if max1.CompareValues(max2) > 0 {
		return 1
	} else {
		// если старшие равны, сравниваем младшие
		min1, min2 := &Card{}, &Card{}
		if firsPair1[0].CompareValues(max1) != 0 {
			min1 = firsPair1[0]
		} else {
			min1 = secondPair1[0]
		}

		if firsPair2[0].CompareValues(max2) != 0 {
			min2 = firsPair2[0]
		} else {
			min2 = secondPair2[0]
		}

		if min1.CompareValues(min2) < 0 {
			return -1
		} else if min1.CompareValues(min2) > 0 {
			return 1
		} else {
			return high1.CompareValues(high2)
		}
	}
}

func compareThrees(first, other *HandRaw) int {
	hand1, _, high1 := first.GetThree()
	hand2, _, high2 := other.GetThree()

	if hand1[0].CompareValues(hand2[0]) < 0 {
		return -1
	} else if hand1[0].CompareValues(hand2[0]) > 0 {
		return 1
	} else {
		return high1.CompareValues(high2)
	}
}

func compareFours(first, other *HandRaw) int {
	hand1 := first.GetFour()
	hand2 := other.GetFour()

	return hand1[0].CompareValues(hand2[0])
}

func compareFullHouses(first, other *HandRaw) int {
	three1, _ := first.GetFullHouse()
	three2, _ := other.GetFullHouse()

	return three1[0].CompareValues(three2[0])
}

func compareStraightsOrFlushs(first, other *HandRaw) int {
	return first.Max().CompareValues(other.Max())
}

func compareSimilarHands(first, other *HandRaw) int {
	firstHand := first.Define()

	firstMax := first.Max()
	otherMax := other.Max()

	switch {
	case firstHand == PairHand:
		return comparePairs(first, other)
	case firstHand == TwoPairHand:
		return compareTwoPairs(first, other)
	case firstHand == ThreeHand:
		return compareThrees(first, other)
	case firstHand == StraightHand || firstHand == FlushHand:
		return compareStraightsOrFlushs(first, other)
	case firstHand == FullHouseHand:
		return compareFullHouses(first, other)
	case firstHand == FourHand:
		return compareFours(first, other)
	case firstHand == StraightFlushHand:
		if firstMax.Value < otherMax.Value {
			return -1
		} else if firstMax.Value > otherMax.Value {
			return 1
		}
	}

	return 0
}

type HandRaw struct {
	cardsMap map[string]int
}

func NewHandFromCards(cards []*Card) *HandRaw {
	m := make(map[CardValue]int)

	for _, card := range cards {
		m[card.Value]++
	}

	h := &HandRaw{
		cardsMap: make(map[string]int),
	}

	for _, card := range cards {
		h.cardsMap[card.String()] = m[card.Value]
	}

	return h
}

func NewHandFromStrings(cards []string) *HandRaw {
	cc := make([]*Card, len(cards))

	for i, c := range cards {
		cc[i] = NewCardFromString(c)
	}

	return NewHandFromCards(cc)
}

func (h *HandRaw) Slice() []*Card {
	cc := make([]*Card, 0)

	for card := range h.cardsMap {
		cc = append(cc, NewCardFromString(card))
	}

	return cc
}

func (h *HandRaw) StringSlice() []string {
	cc := make([]string, 0)

	for card := range h.cardsMap {
		cc = append(cc, card)
	}

	return cc
}

func (h *HandRaw) Define() Hand {
	isStraight := h.HasStraight()
	isFlush := h.HasFlush()

	if isStraight && isFlush && isFlush != false {
		if h.Max().Value == Ace {
			return RoyalFlushHand
		} else {
			return StraightFlushHand
		}
	}

	if isFlush {
		return FlushHand
	}

	if isStraight {
		return StraightFlushHand
	}

	ok := h.HasThree()

	if ok {
		ok = h.CountPairs() > 0
		if ok {
			return FullHouseHand
		}

		return ThreeHand
	}

	if h.HasFour() {
		return FourHand
	}

	if h.CountPairs()/2 == 2 {
		return TwoPairHand
	}

	if h.CountPairs()/2 == 1 {
		return PairHand
	}

	return HighCardHand
}

func (h *HandRaw) Compare(other *HandRaw) int {
	firstHand, otherHand := h.Define(), other.Define()

	if firstHand-otherHand == 0 {
		return compareSimilarHands(h, other)
	} else {
		return int(firstHand - otherHand)
	}
}

func (h *HandRaw) CountPairs() int {
	count := 0

	for _, num := range h.cardsMap {
		if num == 2 {
			count++
		}
	}

	return count
}

func (h *HandRaw) GetPair() (hand []*Card, remains []*Card, high *Card) {
	for card, num := range h.cardsMap {
		if num == 2 {
			hand = append(hand, NewCardFromString(card))
		} else {
			remains = append(remains, NewCardFromString(card))
		}
	}

	high = max(remains)

	remains = deleteCard(remains, high)

	return hand, remains, high
}

func (h *HandRaw) GetTwoPair() (one []*Card, other []*Card, high *Card) {
	for card, num := range h.cardsMap {
		if num == 2 {
			if len(one) == 0 {
				one = append(one, NewCardFromString(card))
			} else if len(one) == 1 {
				if one[0].CompareValues(NewCardFromString(card)) == 0 {
					one = append(one, NewCardFromString(card))
				} else {
					other = append(other, NewCardFromString(card))
				}
			} else {
				other = append(other, NewCardFromString(card))
			}
		} else {
			high = NewCardFromString(card)
		}
	}

	return one, other, high
}

func (h *HandRaw) HasThree() bool {
	for _, num := range h.cardsMap {
		if num == 3 {
			return true
		}
	}

	return false
}

func (h *HandRaw) GetThree() (hand []*Card, remains []*Card, high *Card) {
	for card, num := range h.cardsMap {
		if num == 3 {
			hand = append(hand, NewCardFromString(card))
		} else {
			remains = append(remains, NewCardFromString(card))
		}
	}

	high = max(remains)

	remains = deleteCard(remains, high)

	return hand, remains, high
}

func (h *HandRaw) HasFour() bool {
	for _, num := range h.cardsMap {
		if num == 4 {
			return true
		}
	}

	return false
}

func (h *HandRaw) GetFour() (hand []*Card) {
	for card, num := range h.cardsMap {
		if num == 4 {
			hand = append(hand, NewCardFromString(card))
		}
	}

	return hand
}

func (h *HandRaw) HasFlush() bool {
	firstSuite := &CardSuite{
		Color: Red,
		Suite: Hearts,
	}

	ok := true

	for card := range h.cardsMap {
		ok = ok && NewCardFromString(card).Suite.Compare(firstSuite)
	}

	return ok
}

func (h *HandRaw) HasStraight() bool {
	cards := h.Slice()

	sort.Slice(cards, func(i, j int) bool {
		return cards[i].Value < cards[j].Value
	})

	ok := true

	for i := 1; i < len(cards); i++ {
		ok = ok && int(cards[i].Value)-int(cards[i-1].Value) == 1
	}

	return ok
}

func (h *HandRaw) HasFullHouse() bool {
	return h.HasThree() && h.CountPairs() > 0
}

func (h *HandRaw) GetFullHouse() (three []*Card, two []*Card) {
	for card, num := range h.cardsMap {
		if num == 3 {
			three = append(three, NewCardFromString(card))
		} else {
			two = append(two, NewCardFromString(card))
		}
	}

	return three, two
}

func (h *HandRaw) Max() *Card {
	cc := make([]*Card, 0)

	for c := range h.cardsMap {
		cc = append(cc, NewCardFromString(c))
	}

	return max(cc)
}

func (h *HandRaw) Same(other *HandRaw) bool {
	for card, num := range h.cardsMap {
		if v, ok := other.cardsMap[card]; !ok && v != num {
			return false
		}
	}

	return true
}

func Compare(one, other *HandRaw) int {
	return one.Compare(other)
}

func GetMaxHandWithBoard(boad, pocket []string) *HandRaw {
	cards := append(boad, pocket...)

	return getMaxHand(cards)
}

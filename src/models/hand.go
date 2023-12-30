package models

import (
	"errors"
	"sort"

	"hands/src/helpers"
)

const (
	// HandSize - количество карт в комбинации
	HandSize = 5

	// PocketSize - кол-во карт на руках у игрока
	PocketSize = 2
)

// HandValue - значение комбинации
type HandValue int

const (
	HighCardHand      HandValue = iota // Старшая карта
	PairHand                           // Пара
	TwoPairHand                        // Две пары
	ThreeHand                          // Три карты
	StraightHand                       // Стрит
	FlushHand                          // Флэш
	FullHouseHand                      // Фулл-хаус
	FourHand                           // Каре
	StraightFlushHand                  // Стрит-флэш
	RoyalFlushHand                     // Роял-флэш
)

/* Операции с наборами карт, представленными как []string и []*Cards */

// getMaxHand определяет максимальную руку из набора комбинаций из 7 карт по 5.
// Возвращает ошибку, если len(cards) != 7
func getMaxHand(cards []string) (*Hand, error) {
	if len(cards) != HandSize+PocketSize {
		return nil, errors.New("Wrong number of cards to calculate combinations")
	}

	combos := helpers.GetCombinations(cards, HandSize)
	hands := make([]*Hand, len(combos))

	for i, c := range combos {
		hands[i] = NewHandFromStrings(c)
	}

	sort.Slice(hands, func(i, j int) bool {
		return hands[i].Compare(hands[j]) < 0
	})

	return hands[len(hands)-1], nil
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

/* Hand */

// Hand  - покерная комбинация из пяти карт.
// Поле cardsMap - мапа, ключи которой - строковые представления карт (QH, 10C),
// а значения - количество карт данного номинала в комбинации. К примеру, для комбинации []string{"10H","10D","AC","6H","3S"}
// это будет
//
//	map[string]int{
//		"10H":"2",
//		"10D":"2",
//		"AC":"1",
//		"6H":"1",
//		"3S":"1"
//	}
type Hand struct {
	cardsMap map[string]int
}

func ValidateHand(hand []*Card) error {
	if len(hand) != 5 {
		return errors.New("Hand must contains 5 cards only")
	}

	if countDuplicates(hand) > 0 {
		return errors.New("Hand must not contains duplicates")
	}

	return nil
}

func NewHandFromCards(cards []*Card) *Hand {
	m := make(map[CardValue]int)

	for _, card := range cards {
		m[card.Value]++
	}

	h := &Hand{
		cardsMap: make(map[string]int),
	}

	for _, card := range cards {
		h.cardsMap[card.String()] = m[card.Value]
	}

	return h
}

func NewHandFromStrings(cards []string) *Hand {
	cc := make([]*Card, len(cards))

	for i, c := range cards {
		cc[i] = NewCardFromString(c)
	}

	return NewHandFromCards(cc)
}

func (h *Hand) Slice() []*Card {
	cc := make([]*Card, 0)

	for card := range h.cardsMap {
		cc = append(cc, NewCardFromString(card))
	}

	return cc
}

func (h *Hand) StringSlice() []string {
	cc := make([]string, 0)

	for card := range h.cardsMap {
		cc = append(cc, card)
	}

	return cc
}

// Define определяет величину руки
func (h *Hand) Define() HandValue {
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

func (h *Hand) Compare(other *Hand) int {
	firstHand, otherHand := h.Define(), other.Define()

	if firstHand-otherHand == 0 {
		return compareSimilarHands(h, other)
	} else {
		return int(firstHand - otherHand)
	}
}

// CountPairs подсчитывает в руке количество пар карт с одинаковыми величинами
func (h *Hand) CountPairs() int {
	count := 0

	for _, num := range h.cardsMap {
		if num == 2 {
			count++
		}
	}

	return count
}

// GetPair получает комбинацию Pair в случае, если рука была ранее определена как Pair.
// Возвращает пару, две карты, не входящие в комбинацию и кикер (старшую карту).
func (h *Hand) GetPair() (hand []*Card, remains []*Card, high *Card) {
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

// GetTwoPair получает комбинацию TwoPair в случае, если рука была ранее определена как TwoPair.
// Возвращает старшую пару, младшую пару и кикер (старшую карту).
func (h *Hand) GetTwoPair() (one []*Card, other []*Card, high *Card) {
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

// HasThree определяет, является ли рука тройкой
func (h *Hand) HasThree() bool {
	for _, num := range h.cardsMap {
		if num == 3 {
			return true
		}
	}

	return false
}

// GetThree получает комбинацию Three в случае, если рука была ранее определена как Three.
// Возвращает тройку, слайс карт с единственным элементом - картой остатка, - и кикер (старшую карту).
func (h *Hand) GetThree() (hand []*Card, remains []*Card, high *Card) {
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

// HasFour определяет, является ли рука Four
func (h *Hand) HasFour() bool {
	for _, num := range h.cardsMap {
		if num == 4 {
			return true
		}
	}

	return false
}

// GetFour получает комбинацию Four в случае, если рука была ранее определена как Four.
// Возвращает слайс из четырех карт
func (h *Hand) GetFour() (hand []*Card) {
	for card, num := range h.cardsMap {
		if num == 4 {
			hand = append(hand, NewCardFromString(card))
		}
	}

	return hand
}

// HasFlush определяет, является ли рука Flush
func (h *Hand) HasFlush() bool {
	firstCard := ""

	for card := range h.cardsMap {
		firstCard = card
		if true {
			break
		}
	}

	ok := true

	for card := range h.cardsMap {
		ok = ok && NewCardFromString(card).Suite.Compare(NewCardFromString(firstCard).Suite)
	}

	return ok
}

// HasStraight определяет, является ли рука стритом
func (h *Hand) HasStraight() bool {
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

// HasFullHouse определяет, является ли рука FH
func (h *Hand) HasFullHouse() bool {
	return h.HasThree() && h.CountPairs() > 0
}

// GetFullHouse получает комбинацию FullHouse в случае, если рука была ранее определена как FullHouse.
// Возвращает тройку и двойку карт, одинаковых по значению.
func (h *Hand) GetFullHouse() (three []*Card, two []*Card) {
	for card, num := range h.cardsMap {
		if num == 3 {
			three = append(three, NewCardFromString(card))
		} else {
			two = append(two, NewCardFromString(card))
		}
	}

	return three, two
}

func (h *Hand) Max() *Card {
	cc := make([]*Card, 0)

	for c := range h.cardsMap {
		cc = append(cc, NewCardFromString(c))
	}

	return max(cc)
}

// Same определяет, входят ли в другую руку те же самые карты.
func (h *Hand) Same(other *Hand) bool {
	for card, num := range h.cardsMap {
		if v, ok := other.cardsMap[card]; !ok && v != num {
			return false
		}
	}

	return true
}

func Compare(one, other *Hand) int {
	return one.Compare(other)
}

func GetMaxHandWithBoard(boad, pocket []string) (*Hand, error) {
	cards := append(boad, pocket...)

	return getMaxHand(cards)
}

/* Функции сравнения рук*/

// Функции сравнения рук возвращают
// -1, если первая комбинация меньше второй, 1, если первая комбинация больше, 0, если они равны.

// comparePairs сравнивает две комбинации Pair
func comparePairs(first, other *Hand) int {
	h1, _, hc1 := first.GetPair()
	h2, _, hc2 := other.GetPair()

	c := h1[0].CompareValues(h2[0])

	if c != 0 {
		return c
	} else {
		return hc1.CompareValues(hc2)
	}
}

// compareTwoPairs сравнивает две комбинации TwoPair
func compareTwoPairs(first, other *Hand) int {
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

// compareThrees сравнивает две комбинации Three
func compareThrees(first, other *Hand) int {
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

// compareFours сравнивает две комбинации Four
func compareFours(first, other *Hand) int {
	hand1 := first.GetFour()
	hand2 := other.GetFour()

	return hand1[0].CompareValues(hand2[0])
}

// compareFullHouses сравнивает две комбинации FullHouse
func compareFullHouses(first, other *Hand) int {
	three1, _ := first.GetFullHouse()
	three2, _ := other.GetFullHouse()

	return three1[0].CompareValues(three2[0])
}

// compareStraightsOrFlushs сравнивает две комбинации Straight или Flush
func compareStraightsOrFlushs(first, other *Hand) int {
	return first.Max().CompareValues(other.Max())
}

// compareSimilarHands сравнивает руки одинаковой величины
func compareSimilarHands(first, other *Hand) int {
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

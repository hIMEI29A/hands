package models

import (
	"math/rand"
	"time"
)

// DeckLength is the number of cards in a full deck.
const DeckLength = 52

// EOF is the message of the EofError.
const EOF = "eof"

// NewEofError returns the error returned when a card is requested from an
// empty deck.
func NewEofError() EofError {
	return EofError{}
}

// EofError - ошибка, возникающая при попытке получения карты из слайса с нулевой длиной.
type EofError struct {
}

// Error returns the error message.
func (e EofError) Error() string {
	return EOF
}

// Deck представляет карточную колоду
type Deck struct {
	cards []*Card
	rng   *rand.Rand
}

// NewDeck returns a Deck with an internal *rand.Rand seeded from time.Now().UnixNano().
func NewDeck() *Deck {
	return &Deck{
		cards: make([]*Card, 0),
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewDeckWithRand returns a Deck driven by the given *rand.Rand, so shuffles are
// fully deterministic when the same seed is used. A nil rng falls back to a
// default source seeded from the current time.
func NewDeckWithRand(rng *rand.Rand) *Deck {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	return &Deck{cards: make([]*Card, 0), rng: rng}
}

// rngSource returns the deck's *rand.Rand, falling back to a default source
// when the field is nil (e.g. a Deck created via a struct literal).
func (d *Deck) rngRand() *rand.Rand {
	if d.rng == nil {
		return rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	return d.rng
}

// makeOrderedCards возвращает []*Card, заполненный картами, упорядоченными по мастям и значениям
//
//	[]*Card{
//		"2H".
//		"2D".
//		"2S".
//		"2C".
//		"3H".
//		"3D".
//		...
//		"AC".
//	}
func makeOrderedCards() []*Card {
	cards := make([]*Card, 0)

	for _, suit := range Suits {
		for _, cardValue := range CardValues {
			cards = append(cards, &Card{
				Value: cardValue,
				Suit: &CardSuit{
					Color: suit.Color(),
					Suit:  suit,
				},
			})
		}
	}

	return cards
}

// Shuffle resets the deck to the full 52-card ordered set and applies an unbiased
// Fisher-Yates shuffle, so it always contains exactly 52 unique non-nil cards.
// It is safe to call repeatedly and always returns a full deck. Возвращает d
func (d *Deck) Shuffle() *Deck {
	cards := makeOrderedCards()

	rng := d.rngRand()

	for i := len(cards) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		cards[i], cards[j] = cards[j], cards[i]
	}

	d.cards = cards

	return d
}

// Len returns the number of cards left in the deck.
func (d *Deck) Len() int {
	return len(d.cards)
}

// pop удаляет последний элемент слайса cards (верхнюю карту в колоде) и возвращает его.
// Возвращает ошибку, если длина cards равна 0.
func (d *Deck) pop() (*Card, error) {
	if len(d.cards) == 0 {
		return nil, NewEofError()
	}

	card, cards := d.cards[len(d.cards)-1], d.cards[0:len(d.cards)-1]

	d.cards = cards

	return card, nil
}

// Card возвращает последнюю карту в колоде, или ошибку в случае, если карт в колоде нет, т.е длина cards равна 0.
func (d *Deck) Card() (*Card, error) {
	return d.pop()
}

// Discard удаляет последнюю карту в колоде без получения ее значения. Возвращает ошибку в случае, если карт в колоде нет, т.е длина cards равна 0.
func (d *Deck) Discard() error {
	_, err := d.pop()

	return err
}

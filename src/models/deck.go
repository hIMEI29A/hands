package models

import "hands/src/helpers"

const DeckLength = 52

const EOF = "eof"

func NewEofError() EofError {
	return EofError{}
}

// EofError - ошибка, возникающая при попытке получения карты из слайса с нулевой длиной.
type EofError struct {
}

func (e EofError) Error() string {
	return EOF
}

// Deck представляет карточную колоду
type Deck struct {
	cards []*Card
}

func NewDeck() *Deck {
	return &Deck{cards: make([]*Card, 0)}
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

	for _, suite := range Suites {
		for _, cardValue := range CardValues {
			cards = append(cards, &Card{
				Value: cardValue,
				Suite: &CardSuite{
					Color: suite.Color(),
					Suite: suite,
				},
			})
		}
	}

	return cards
}

// Shuffle заполняет слайс d.cards уникальными случайными значениями из полного упорядоченного набора карт.
// Возвращает d
func (d *Deck) Shuffle() *Deck {
	control := make(map[int]struct{})
	orderedCards := makeOrderedCards()

	d.cards = make([]*Card, len(orderedCards))

	for i := 0; i < len(orderedCards); i++ {
		num := helpers.GenerateRandomNumInRange(len(orderedCards))

		if _, ok := control[num]; !ok {
			control[num] = struct{}{}

			d.cards[i] = orderedCards[num]
		}
	}

	return d
}

func (d *Deck) randomCard() *Card {
	return NewRandomCard()
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

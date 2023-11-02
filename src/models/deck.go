package models

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

func NewDeck() *Deck {
	return &Deck{cards: make([]*Card, 0)}
}

func Shuffle() *Deck {
	seen := make(map[string]struct{})

	deck := &Deck{
		cards: make([]*Card, 0),
	}

	for i := 0; i < DeckLength; i++ {
		card := deck.randomCard()
		if _, ok := seen[card.String()]; !ok {
			deck.cards = append(deck.cards, card)
			seen[card.String()] = struct{}{}
		}
	}

	return deck
}

// Deck представляет карточную колоду
type Deck struct {
	cards []*Card
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

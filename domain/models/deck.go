package models

const DeckLength = 52

const EOF = "eof"

func NewEofError() EofError {
	return EofError{}
}

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

type Deck struct {
	cards []*Card
}

func (d *Deck) randomCard() *Card {
	return NewRandomCard()
}

func (d *Deck) pop() (*Card, error) {
	if len(d.cards) == 0 {
		return nil, NewEofError()
	}

	card, cards := d.cards[len(d.cards)-1], d.cards[0:len(d.cards)-1]

	d.cards = cards

	return card, nil
}

func (d *Deck) Card() (*Card, error) {
	return d.pop()
}

func (d *Deck) Discard() error {
	_, err := d.pop()

	return err
}

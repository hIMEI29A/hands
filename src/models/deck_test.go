package models

import (
	"errors"
	"math/rand"
	"testing"
)

func TestShuffleFullUniqueDeck(t *testing.T) {
	d := NewDeckWithRand(rand.New(rand.NewSource(1))).Shuffle()

	if got, want := d.Len(), DeckLength; got != want {
		t.Fatalf("after Shuffle: Len() = %d, want %d", got, want)
	}

	seen := make(map[string]struct{}, DeckLength)

	for i, c := range d.cards {
		if c == nil {
			t.Fatalf("after Shuffle: cards[%d] is nil", i)
		}

		if _, ok := seen[c.String()]; ok {
			t.Errorf("after Shuffle: duplicate card %q", c.String())
		}

		seen[c.String()] = struct{}{}
	}

	if len(seen) != DeckLength {
		t.Errorf("after Shuffle: %d unique cards, want %d", len(seen), DeckLength)
	}
}

func TestShuffleSameSeedSameOrder(t *testing.T) {
	seed := int64(42)

	d1 := NewDeckWithRand(rand.New(rand.NewSource(seed))).Shuffle()
	d2 := NewDeckWithRand(rand.New(rand.NewSource(seed))).Shuffle()

	s1 := NewStringSliceFromCards(d1.cards)
	s2 := NewStringSliceFromCards(d2.cards)

	if len(s1) != len(s2) {
		t.Fatalf("deck lengths differ: %d vs %d", len(s1), len(s2))
	}

	for i := range s1 {
		if s1[i] != s2[i] {
			t.Fatalf("decks with the same seed differ at index %d: %q vs %q", i, s1[i], s2[i])
		}
	}
}

func TestShuffleDifferentSeedsDiffer(t *testing.T) {
	d1 := NewDeckWithRand(rand.New(rand.NewSource(1))).Shuffle()
	d2 := NewDeckWithRand(rand.New(rand.NewSource(2))).Shuffle()

	s1 := NewStringSliceFromCards(d1.cards)
	s2 := NewStringSliceFromCards(d2.cards)

	differ := false

	for i := range s1 {
		if s1[i] != s2[i] {
			differ = true

			break
		}
	}

	if !differ {
		t.Error("decks with different seeds produced the same order")
	}
}

func TestShuffleRepeated(t *testing.T) {
	d := NewDeckWithRand(rand.New(rand.NewSource(7)))

	for round := 0; round < 5; round++ {
		d.Shuffle()

		if got, want := d.Len(), DeckLength; got != want {
			t.Fatalf("round %d: Len() = %d, want %d", round, got, want)
		}

		seen := make(map[string]struct{}, DeckLength)

		for i, c := range d.cards {
			if c == nil {
				t.Fatalf("round %d: cards[%d] is nil", round, i)
			}

			if _, ok := seen[c.String()]; ok {
				t.Errorf("round %d: duplicate card %q", round, c.String())
			}

			seen[c.String()] = struct{}{}
		}

		if len(seen) != DeckLength {
			t.Errorf("round %d: %d unique cards, want %d", round, len(seen), DeckLength)
		}
	}
}

func TestShuffleResetsPartiallyDealtDeck(t *testing.T) {
	d := NewDeckWithRand(rand.New(rand.NewSource(3))).Shuffle()

	// Deal away most of the deck, then shuffle again.
	for i := 0; i < DeckLength-1; i++ {
		if _, err := d.Card(); err != nil {
			t.Fatalf("Card() %d returned error: %v", i, err)
		}
	}

	if got := d.Len(); got != 1 {
		t.Fatalf("after dealing 51 cards: Len() = %d, want 1", got)
	}

	d.Shuffle()

	if got, want := d.Len(), DeckLength; got != want {
		t.Errorf("after re-Shuffle: Len() = %d, want %d", got, want)
	}
}

func TestDealAllCardsThenEofError(t *testing.T) {
	d := NewDeckWithRand(rand.New(rand.NewSource(11))).Shuffle()

	for i := 0; i < DeckLength; i++ {
		c, err := d.Card()
		if err != nil {
			t.Fatalf("Card() %d returned error: %v", i, err)
		}

		if c == nil {
			t.Fatalf("Card() %d returned nil card", i)
		}
	}

	if got, want := d.Len(), 0; got != want {
		t.Fatalf("after dealing all cards: Len() = %d, want %d", got, want)
	}

	var eofErr EofError

	_, err := d.Card()
	if err == nil {
		t.Fatal("Card() on empty deck returned nil error, want EofError")
	}

	if !errors.Is(err, eofErr) {
		t.Errorf("Card() on empty deck returned %v, want EofError", err)
	}
}

func TestDiscardOnEmptyDeck(t *testing.T) {
	d := NewDeckWithRand(rand.New(rand.NewSource(13))).Shuffle()

	if err := d.Discard(); err != nil {
		t.Fatalf("Discard() on full deck returned error: %v", err)
	}

	if got, want := d.Len(), DeckLength-1; got != want {
		t.Errorf("after one Discard: Len() = %d, want %d", got, want)
	}

	var eofErr EofError

	// Discard the rest of the deck.
	for i := 1; i < DeckLength; i++ {
		if err := d.Discard(); err != nil {
			t.Fatalf("Discard() %d returned error: %v", i, err)
		}
	}

	if got := d.Len(); got != 0 {
		t.Fatalf("after discarding all cards: Len() = %d, want 0", got)
	}

	// Deal one more first: Card() leaves the deck empty, then Discard must fail.
	_, err := d.Card()
	if err == nil {
		t.Fatal("Card() on empty deck returned nil error, want EofError")
	}

	if !errors.Is(err, eofErr) {
		t.Errorf("Card() on empty deck returned %v, want EofError", err)
	}

	if got := d.Len(); got != 0 {
		t.Fatalf("Len() = %d after failed Card(), want 0", got)
	}

	err = d.Discard()
	if err == nil {
		t.Fatal("Discard() on empty deck returned nil error, want EofError")
	}

	if !errors.Is(err, eofErr) {
		t.Errorf("Discard() on empty deck returned %v, want EofError", err)
	}
}

func TestNewDeckAndNilRandFallback(t *testing.T) {
	d := NewDeck()
	if d.rng == nil {
		t.Fatal("NewDeck() left rng nil")
	}

	if c := d.Shuffle(); c.Len() != DeckLength {
		t.Errorf("NewDeck().Shuffle(): Len() = %d, want %d", c.Len(), DeckLength)
	}

	// nil rng must be guarded against.
	d2 := NewDeckWithRand(nil)
	if d2.rng == nil {
		t.Fatal("NewDeckWithRand(nil) left rng nil")
	}

	if got, want := d2.Shuffle().Len(), DeckLength; got != want {
		t.Errorf("after Shuffle with nil-rng fallback: Len() = %d, want %d", got, want)
	}
}

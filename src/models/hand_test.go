package models

import (
	"reflect"
	"testing"
)

// mustCards parses card strings for the tests or fails the test.
func mustCards(t *testing.T, ss ...string) []*Card {
	t.Helper()

	cc := make([]*Card, len(ss))
	for i, s := range ss {
		c, err := ParseCard(s)
		if err != nil {
			t.Fatalf("bad card string %q: %v", s, err)
		}

		cc[i] = c
	}

	return cc
}

// mustHand builds a Hand from card strings or fails the test.
func mustHand(t *testing.T, cards []string) *Hand {
	t.Helper()

	h, err := NewHandFromStrings(cards)
	if err != nil {
		t.Fatalf("NewHandFromStrings(%v): %v", cards, err)
	}

	return h
}

// sign reduces an int to its -1/0/1 sign.
func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}

	return 0
}

func TestDefine(t *testing.T) {
	tests := []struct {
		name  string
		cards []string
		want  HandValue
	}{
		{"high card", []string{"2C", "5H", "9D", "JS", "KD"}, HighCardHand},
		{"pair", []string{"2C", "2H", "9D", "JS", "KD"}, PairHand},
		{"two pair", []string{"2C", "2H", "9D", "9S", "KD"}, TwoPairHand},
		{"three", []string{"2C", "2H", "2D", "JS", "KD"}, ThreeHand},
		{"straight", []string{"5H", "6C", "7D", "8S", "9C"}, StraightHand},
		{"wheel straight", []string{"AH", "2C", "3D", "4S", "5C"}, StraightHand},
		{"ace-high straight", []string{"10H", "JC", "QD", "KS", "AC"}, StraightHand},
		{"flush", []string{"2C", "5C", "9C", "JC", "KC"}, FlushHand},
		{"full house", []string{"2C", "2H", "2D", "JS", "JC"}, FullHouseHand},
		{"four of a kind", []string{"2C", "2H", "2D", "2S", "KD"}, FourHand},
		{"straight flush", []string{"6H", "7H", "8H", "9H", "10H"}, StraightFlushHand},
		{"wheel straight flush", []string{"AH", "2H", "3H", "4H", "5H"}, StraightFlushHand},
		{"royal flush", []string{"10D", "JD", "QD", "KD", "AD"}, RoyalFlushHand},
	}

	for _, tt := range tests {
		h := mustHand(t, tt.cards)
		if got := h.Define(); got != tt.want {
			t.Errorf("%s: Define() = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestEvaluateTiebreaks(t *testing.T) {
	tests := []struct {
		name  string
		cards []string
		want  []CardValue
	}{
		{"high card descending", []string{"KD", "9H", "7C", "6S", "2C"}, []CardValue{King, Nine, Seven, Six, Two}},
		{"pair with kickers", []string{"2C", "2S", "7D", "8H", "9D"}, []CardValue{Two, Nine, Eight, Seven}},
		{"two pairs with kicker", []string{"3C", "3D", "2C", "2H", "AS"}, []CardValue{Three, Two, Ace}},
		{"two pairs descending pairs", []string{"9C", "9D", "5C", "5H", "AS"}, []CardValue{Nine, Five, Ace}},
		{"trip with kickers", []string{"5C", "5D", "5H", "8S", "AH"}, []CardValue{Five, Ace, Eight}},
		{"straight high = 9", []string{"5H", "6C", "7D", "8S", "9C"}, []CardValue{Nine}},
		{"wheel straight high = 5", []string{"AH", "2C", "3D", "4S", "5C"}, []CardValue{Five}},
		{"flush descending", []string{"KC", "9C", "5C", "2C", "JC"}, []CardValue{King, Jack, Nine, Five, Two}},
		{"straight flush high", []string{"6H", "7H", "8H", "9H", "10H"}, []CardValue{Ten}},
		{"wheel straight flush high = 5", []string{"AH", "2H", "3H", "4H", "5H"}, []CardValue{Five}},
		{"full house trip over pair", []string{"5C", "5D", "5H", "8S", "8H"}, []CardValue{Five, Eight}},
		{"four of a kind with kicker", []string{"5C", "5D", "5H", "5S", "AH"}, []CardValue{Five, Ace}},
	}

	for _, tt := range tests {
		_, tb := evaluate(mustCards(t, tt.cards...))
		if !reflect.DeepEqual(tb, tt.want) {
			t.Errorf("%s: tiebreaks = %v, want %v", tt.name, tb, tt.want)
		}
	}

	_, tbRoyal := evaluate(mustCards(t, "10D", "JD", "QD", "KD", "AD"))
	if len(tbRoyal) != 0 {
		t.Errorf("royal flush tiebreaks = %v, want empty", tbRoyal)
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want int // sign of Compare(a, b)
	}{
		{"pair beats high card", []string{"2C", "2H", "9D", "JS", "KD"}, []string{"2C", "5H", "9D", "JS", "KD"}, 1},
		{"two pair beats pair", []string{"2C", "2H", "9D", "9S", "KD"}, []string{"AC", "AH", "5D", "JS", "KD"}, 1},
		{"trip beats two pair", []string{"2C", "2H", "2D", "JS", "KD"}, []string{"2C", "2H", "9D", "9S", "KD"}, 1},
		{"straight beats trip", []string{"5H", "6C", "7D", "8S", "9C"}, []string{"2C", "2H", "2D", "JS", "KD"}, 1},
		{"6-high straight beats wheel", []string{"2C", "3D", "4S", "5C", "6H"}, []string{"AH", "2C", "3D", "4S", "5C"}, 1},
		{"straight loses by high card", []string{"4H", "5C", "6D", "7S", "8C"}, []string{"5H", "6C", "7D", "8S", "9C"}, -1},
		{"flush beats straight", []string{"2C", "5C", "9C", "JC", "KC"}, []string{"5H", "6C", "7D", "8S", "9C"}, 1},
		{"full house beats flush", []string{"2C", "2H", "2D", "JS", "JC"}, []string{"2C", "5C", "9C", "JC", "KC"}, 1},
		{"four beats full house", []string{"2C", "2H", "2D", "2S", "KD"}, []string{"5C", "5D", "5H", "8S", "8H"}, 1},
		{"straight flush beats four", []string{"6H", "7H", "8H", "9H", "10H"}, []string{"2C", "2H", "2D", "2S", "KD"}, 1},
		{"royal flush beats straight flush", []string{"10D", "JD", "QD", "KD", "AD"}, []string{"6H", "7H", "8H", "9H", "10H"}, 1},
		{"wheel straight flush beats wheel straight", []string{"AH", "2H", "3H", "4H", "5H"}, []string{"AH", "2C", "3D", "4S", "5C"}, 1},
		{"two pair high pair decides", []string{"2C", "2H", "5D", "5S", "AH"}, []string{"3C", "3D", "4H", "4S", "KH"}, 1},
		{"pair value decides", []string{"AC", "AH", "2D", "3S", "4C"}, []string{"KC", "KH", "AD", "QS", "JC"}, 1},
		{"pair kicker decides", []string{"2C", "2S", "7D", "8H", "9D"}, []string{"2C", "2S", "7D", "8H", "6C"}, 1},
		{"pair earlier kicker decides", []string{"2C", "2S", "6D", "8H", "9D"}, []string{"2C", "2S", "7D", "8H", "9D"}, -1},
		{"trip kicker decides", []string{"5C", "5D", "5H", "8S", "AH"}, []string{"5C", "5D", "5H", "7S", "AH"}, 1},
		{"flush second card decides", []string{"2H", "5H", "7H", "9H", "JH"}, []string{"2D", "5D", "7D", "9D", "10D"}, 1},
		{"four value decides", []string{"2C", "2H", "2D", "2S", "AH"}, []string{"3C", "3D", "3H", "3S", "2C"}, -1},
		{"full house trip decides", []string{"2C", "2H", "2D", "JS", "JC"}, []string{"AC", "AD", "AH", "KS", "KH"}, -1},
		{"full house pair decides", []string{"AC", "AD", "AH", "KS", "KH"}, []string{"AC", "AD", "AH", "QS", "QH"}, 1},
		{"equal hands different suits", []string{"2C", "2H", "9D", "JS", "KD"}, []string{"2S", "2D", "9C", "JH", "KH"}, 0},
		{"equal flushes different suits", []string{"2C", "5C", "9C", "JC", "KC"}, []string{"2D", "5D", "9D", "JD", "KD"}, 0},
		{"identical hands", []string{"2C", "3D", "4S", "5H", "6D"}, []string{"2C", "3D", "4S", "5H", "6D"}, 0},
	}

	for _, tt := range tests {
		a, b := mustHand(t, tt.a), mustHand(t, tt.b)

		if got := sign(a.Compare(b)); got != tt.want {
			t.Errorf("%s: Compare(a, b) = %d, want %+d", tt.name, got, tt.want)
		}

		if back := sign(b.Compare(a)); back != -tt.want {
			t.Errorf("%s: Compare(b, a) = %d, want %+d", tt.name, back, -tt.want)
		}
	}
}

// TestCompareAntisymmetry checks that Compare is a total order over a set
// of mixed hands: antisymmetric in both directions, irreflexive except
// for equal hands, and transitive.
func TestCompareAntisymmetry(t *testing.T) {
	set := [][]string{
		{"2C", "2H", "2D", "2S", "KD"},  // four
		{"2C", "2H", "2D", "JS", "JC"},  // full house
		{"AC", "AD", "AH", "KS", "KH"},  // full house
		{"2C", "5C", "9C", "JC", "KC"},  // flush
		{"6H", "7H", "8H", "9H", "10H"}, // straight flush
		{"10D", "JD", "QD", "KD", "AD"}, // royal flush
		{"2C", "2H", "2D", "JS", "KD"},  // trips
		{"AC", "AH", "5D", "JS", "KD"},  // pair
		{"KC", "KH", "AD", "QS", "JC"},  // pair
		{"2C", "2H", "9D", "9S", "KD"},  // two pair
		{"3C", "3D", "4H", "4S", "KH"},  // two pair
		{"5H", "6C", "7D", "8S", "9C"},  // straight
		{"AH", "2C", "3D", "4S", "5C"},  // wheel straight
		{"2C", "5H", "9D", "JS", "KD"},  // high card
		{"2C", "2H", "9D", "JS", "KD"},  // different pair
		{"2S", "2D", "9C", "JH", "KH"},  // equal to deepest pair
		{"2C", "2H", "9D", "JS", "KD"},  // duplicate of the pair
	}

	hands := make([]*Hand, len(set))
	for i, cards := range set {
		hands[i] = mustHand(t, cards)
	}

	for i, a := range hands {
		for j, b := range hands {
			if sign(a.Compare(b)) != -sign(b.Compare(a)) {
				t.Errorf("hands[%d] vs hands[%d]: not antisymmetric", i, j)
			}

			if i == j && a.Compare(b) != 0 {
				t.Errorf("hands[%d]: Compare(h, h) = %d, want 0", i, a.Compare(b))
			}
		}
	}

	// Transitivity on all triples: a <= b and b <= c imply a <= c.
	for i, a := range hands {
		for j, b := range hands {
			for k, c := range hands {
				if sign(a.Compare(b)) <= 0 && sign(b.Compare(c)) <= 0 && sign(a.Compare(c)) > 0 {
					t.Errorf("hands[%d] <= hands[%d] <= hands[%d] but hands[%d] > hands[%d]", i, j, k, i, k)
				}
			}
		}
	}
}

func TestSame(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{"same cards different order", []string{"KD", "2C", "9H", "7S", "3D"}, []string{"2C", "3D", "7S", "9H", "KD"}, true},
		{"different cards", []string{"KD", "2C", "9H", "7S", "3D"}, []string{"KC", "2C", "9H", "7S", "3D"}, false},
	}

	for _, tt := range tests {
		a, b := mustHand(t, tt.a), mustHand(t, tt.b)
		if got := a.Same(b); got != tt.want {
			t.Errorf("%s: Same() = %v, want %v", tt.name, got, tt.want)
		}
	}

	a := mustHand(t, []string{"KD", "2C", "9H", "7S", "3D"})
	if !a.Same(a) {
		t.Error("Same(h, h) = false, want true")
	}

	if a.Same(nil) {
		t.Error("Same(h, nil) = true, want false")
	}
}

func TestHandAccessors(t *testing.T) {
	in := mustCards(t, "KD", "2C", "9H", "7S", "3D")

	h, err := NewHandFromCards(in)
	if err != nil {
		t.Fatalf("NewHandFromCards: %v", err)
	}

	want := []string{"2C", "3D", "7S", "9H", "KD"}
	if got := h.StringSlice(); !reflect.DeepEqual(got, want) {
		t.Errorf("StringSlice() = %v, want sorted %v", got, want)
	}

	slice := h.Slice()
	if len(slice) != HandSize {
		t.Fatalf("Slice() len = %d, want %d", len(slice), HandSize)
	}

	for i := range slice {
		if slice[i].String() != want[i] {
			t.Errorf("Slice()[%d] = %v, want %v", i, slice[i], want[i])
		}
	}

	if got := h.Max().String(); got != "KD" {
		t.Errorf("Max() = %q, want KD", got)
	}

	// The input slice must not be mutated by construction.
	inStrings := NewStringSliceFromCards(in)
	if !reflect.DeepEqual(inStrings, []string{"KD", "2C", "9H", "7S", "3D"}) {
		t.Errorf("input slice mutated: %v", inStrings)
	}
}

func TestValidateHandErrors(t *testing.T) {
	invalid := [][]*Card{
		nil,
		mustCards(t, "2C"),
		mustCards(t, "2C", "3D"),
		mustCards(t, "2C", "3D", "4S", "5H"),
		append(mustCards(t, "2C", "3D", "4S", "5H", "6D"), mustCards(t, "7S")...),
		append(mustCards(t, "2C", "3D", "4S", "5H", "6D"), nil),
		mustCards(t, "2C", "3D", "4S", "5H", "5H"), // duplicate
	}

	for i, cards := range invalid {
		if err := ValidateHand(cards); err == nil {
			t.Errorf("case %d: ValidateHand(%v) = nil, want error", i, cardStrings(cards))
		}

		if _, err := NewHandFromCards(cards); err == nil {
			t.Errorf("case %d: NewHandFromCards returned nil error, want error", i)
		}
	}

	if err := ValidateHand(mustCards(t, "2C", "3D", "4S", "5H", "6D")); err != nil {
		t.Errorf("valid hand rejected: %v", err)
	}
}

func TestNewHandFromStringsErrors(t *testing.T) {
	for _, ss := range [][]string{
		{"XH", "2C", "3D", "4S", "5H"},  // bad suit
		{"1H", "2C", "3D", "4S", "5H"},  // bad value
		{"11H", "2C", "3D", "4S", "5H"}, // bad value
		{"AHx", "2C", "3D", "4S", "5H"}, // trailing junk
		{"", "2C", "3D", "4S", "5H"},    // empty card string
		{"10H"},                         // wrong count
	} {
		if _, err := NewHandFromStrings(ss); err == nil {
			t.Errorf("NewHandFromStrings(%v): want error", ss)
		}
	}

	// Lowercase input is accepted and normalized.
	h, err := NewHandFromStrings([]string{"2c", "3d", "4s", "5h", "6H"})
	if err != nil {
		t.Fatalf("NewHandFromStrings lowercase: %v", err)
	}

	if got := h.Define(); got != StraightHand {
		t.Errorf("lowercase straight Define() = %d, want %d", got, StraightHand)
	}

	// A duplicate after case normalization is rejected.
	if _, err := NewHandFromStrings([]string{"2c", "3d", "4s", "5h", "2C"}); err == nil {
		t.Error("NewHandFromStrings: want duplicate error")
	}
}

func cardStrings(cards []*Card) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		if c == nil {
			out[i] = "<nil>"
			continue
		}

		out[i] = c.String()
	}

	return out
}

func TestGetMaxHandWithBoard(t *testing.T) {
	tests := []struct {
		name   string
		board  []string
		pocket []string
		want   HandValue
	}{
		{
			name:   "9s over kings full house beats single big pair",
			board:  []string{"2H", "2C", "KD", "9S", "9H"},
			pocket: []string{"9C", "KS"},
			want:   FullHouseHand,
		},
		{
			name:   "royal flush on board plus pocket",
			board:  []string{"10S", "QS", "KS", "2H", "3C"},
			pocket: []string{"AS", "JS"},
			want:   RoyalFlushHand,
		},
		{
			name:   "ace-high flush best of 7",
			board:  []string{"2H", "5H", "9H", "JH", "KD"},
			pocket: []string{"AH", "4C"},
			want:   FlushHand,
		},
	}

	for _, tt := range tests {
		h, err := GetMaxHandWithBoard(tt.board, tt.pocket)
		if err != nil {
			t.Fatalf("%s: GetMaxHandWithBoard: %v", tt.name, err)
		}

		if got := h.Define(); got != tt.want {
			t.Errorf("%s: Define() = %d, want %d", tt.name, got, tt.want)
		}
	}

	// Wrong total card count must be an error.
	if _, err := GetMaxHandWithBoard([]string{"2H", "5H", "9D"}, []string{"JS", "KD"}); err == nil {
		t.Error("GetMaxHandWithBoard with 5 cards: want error")
	}

	if _, err := GetMaxHandWithBoard([]string{"2H", "5H", "9D", "JS", "KD", "2C"}, []string{"AS", "3D"}); err == nil {
		t.Error("GetMaxHandWithBoard with 8 cards: want error")
	}
}

// TestExhaustiveCategoryCounts enumerates all C(52,5) five-card hands and
// checks each category count against known combinatorics. Five nested
// loops generate the combinations without gonum. Skipped with -short.
func TestExhaustiveCategoryCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("exhaustive enumeration skipped in short mode")
	}

	deck := make([]*Card, 0, SuitsNum*ValuesNum)
	for _, s := range Suits {
		for _, v := range CardValues {
			deck = append(deck, &Card{Value: v, Suit: &CardSuit{Color: s.Color(), Suit: s}})
		}
	}

	want := map[HandValue]int{
		HighCardHand:      1302540,
		PairHand:          1098240,
		TwoPairHand:       123552,
		ThreeHand:         54912,
		StraightHand:      10200,
		FlushHand:         5108,
		FullHouseHand:     3744,
		FourHand:          624,
		StraightFlushHand: 36,
		RoyalFlushHand:    4,
	}

	got := make(map[HandValue]int, len(want))
	sub := make([]*Card, HandSize)
	total := 0

	n := len(deck)
	for a := 0; a < n; a++ {
		sub[0] = deck[a]
		for b := a + 1; b < n; b++ {
			sub[1] = deck[b]
			for c := b + 1; c < n; c++ {
				sub[2] = deck[c]
				for d := c + 1; d < n; d++ {
					sub[3] = deck[d]
					for e := d + 1; e < n; e++ {
						sub[4] = deck[e]

						got[(&Hand{cards: sub}).Define()]++
						total++
					}
				}
			}
		}
	}

	if total != 2598960 {
		t.Errorf("total hands = %d, want 2598960", total)
	}

	for cat, wantCount := range want {
		if got[cat] != wantCount {
			t.Errorf("category %d count = %d, want %d", cat, got[cat], wantCount)
		}
	}

	if royal, expected := got[RoyalFlushHand], 4; royal != expected {
		t.Errorf("royal flushes = %d, want %d", royal, expected)
	}
}

func TestGetMaxHandWithBoardDoesNotMutateInput(t *testing.T) {
	// Extra capacity makes an accidental append visible in the backing array.
	board := make([]string, 5, 7)
	copy(board, []string{"2H", "3H", "4H", "5H", "7C"})

	backing := board[:cap(board)]
	before := append([]string(nil), backing...)

	pocket := []string{"6H", "8D"}

	if _, err := GetMaxHandWithBoard(board, pocket); err != nil {
		t.Fatalf("GetMaxHandWithBoard returned error: %v", err)
	}

	for i := range backing {
		if backing[i] != before[i] {
			t.Errorf("board backing array changed at index %d: got %q, want %q", i, backing[i], before[i])
		}
	}
}

func TestValidateHandRejectsNilSuit(t *testing.T) {
	good := []*Card{
		NewCardFromString("2H"),
		NewCardFromString("3H"),
		NewCardFromString("4H"),
		NewCardFromString("5H"),
		NewCardFromString("7C"),
	}

	good[2] = &Card{Value: Four}

	if err := ValidateHand(good); err == nil {
		t.Fatal("ValidateHand accepted a card with nil suit, want error")
	}

	if _, err := NewHandFromCards(good); err == nil {
		t.Fatal("NewHandFromCards accepted a card with nil suit, want error")
	}
}

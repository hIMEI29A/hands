package models

import (
	"fmt"
	"testing"
)

func TestParseCardAllValidCards(t *testing.T) {
	for _, suit := range Suits {
		for _, value := range CardValues {
			s := value.String() + string(suit)

			t.Run(s, func(t *testing.T) {
				c, err := ParseCard(s)
				if err != nil {
					t.Fatalf("ParseCard(%q) returned error: %v", s, err)
				}

				if c.Value != value {
					t.Errorf("ParseCard(%q).Value = %v, want %v", s, c.Value, value)
				}

				if c.Suit == nil {
					t.Fatalf("ParseCard(%q).Suit is nil", s)
				}

				if c.Suit.Suit != suit {
					t.Errorf("ParseCard(%q).Suit.Suit = %q, want %q", s, c.Suit.Suit, suit)
				}

				if got := c.String(); got != s {
					t.Errorf("ParseCard(%q).String() = %q, want %q", s, got, s)
				}
			})
		}
	}
}

func TestParseCardSpecificValidCards(t *testing.T) {
	testCases := []struct {
		input string
		value CardValue
		suit  Suit
	}{
		{"10H", Ten, Hearts},
		{"2C", Two, Clubs},
		{"AS", Ace, Spades},
		{"10h", Ten, Hearts},
		{"as", Ace, Spades},
		{"jd", Jack, Diamonds},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			c, err := ParseCard(tc.input)
			if err != nil {
				t.Fatalf("ParseCard(%q) returned error: %v", tc.input, err)
			}

			if c.Value != tc.value {
				t.Errorf("ParseCard(%q).Value = %v, want %v", tc.input, c.Value, tc.value)
			}

			if c.Suit.Suit != tc.suit {
				t.Errorf("ParseCard(%q).Suit.Suit = %q, want %q", tc.input, c.Suit.Suit, tc.suit)
			}
		})
	}
}

func TestParseCardInvalid(t *testing.T) {
	testCases := []string{
		"",     // empty
		"1H",   // no 1 value
		"11H",  // no 11 value
		"AHx",  // trailing junk
		"2H ",  // trailing space
		" 2H",  // leading space
		"2HX",  // unknown suit
		"H",    // suit only
		"A",    // value only
		"2",    // value only
		"J2",   // invalid suit position
		"2XXH", // junk before suit
		"ККH",  // non-ASCII junk
		"2\tH", // whitespace inside
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc), func(t *testing.T) {
			c, err := ParseCard(tc)
			if err == nil {
				t.Fatalf("ParseCard(%q) = %+v, want error", tc, c)
			}

			if c != nil {
				t.Errorf("ParseCard(%q) returned non-nil card %v with error %v", tc, c, err)
			}

			if err.Error() == "" {
				t.Errorf("ParseCard(%q) error message is empty", tc)
			}
		})
	}
}

func TestCardValueStringRoundTrip(t *testing.T) {
	// cardValuesStrings[i] corresponds to CardValue(i + 2).
	for i, want := range cardValuesStrings {
		cv := CardValue(i + 2)

		if got := cv.String(); got != want {
			t.Errorf("CardValue(%d).String() = %q, want %q", int(cv), got, want)
		}
	}

	// The strings produced by String() must be valid value parts of card strings.
	for _, cv := range CardValues {
		if _, err := ParseCard(cv.String() + "H"); err != nil {
			t.Errorf("round-trip of CardValue(%d): ParseCard(%q) returned error: %v", int(cv), cv.String(), err)
		}
	}
}

func TestCardValueStringOutOfRangeDoesNotPanic(t *testing.T) {
	testCases := []struct {
		cv   CardValue
		want string
	}{
		{CardValue(0), "CardValue(0)"},
		{CardValue(1), "CardValue(1)"},
		{CardValue(99), "CardValue(99)"},
		{CardValue(-5), "CardValue(-5)"},
	}

	for _, tc := range testCases {
		if got := tc.cv.String(); got != tc.want {
			t.Errorf("CardValue(%d).String() = %q, want %q", int(tc.cv), got, tc.want)
		}
	}
}

func TestNewCardFromStringPanicsOnInvalid(t *testing.T) {
	testCases := []struct {
		input string
	}{
		{""},    // empty
		{"1H"},  // no 1 value
		{"11H"}, // no 11 value
		{"AHx"}, // trailing junk
		{"2H "}, // trailing space
		{"H"},   // suit only
		{"A"},   // value only
		{"2"},   // value only
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.input), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("NewCardFromString(%q) did not panic", tc.input)
				}
			}()

			NewCardFromString(tc.input)
		})
	}
}

func TestCardSameValue(t *testing.T) {
	testCases := []struct {
		a    string
		b    string
		want bool
	}{
		{"2H", "2C", true},
		{"2H", "3H", false},
		{"10H", "10C", true},
		{"AS", "KS", false},
	}

	for _, tc := range testCases {
		a, b := mustCards(t, tc.a)[0], mustCards(t, tc.b)[0]

		if got := a.SameValue(b); got != tc.want {
			t.Errorf("%s.SameValue(%s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}

		if got := a.EqualTo(b); got != tc.want {
			t.Errorf("%s.EqualTo(%s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCardSameSuit(t *testing.T) {
	testCases := []struct {
		a    string
		b    string
		want bool
	}{
		{"2H", "3H", true},
		{"2H", "2C", false},
		{"AS", "KS", true},
		{"AD", "2D", true},
	}

	for _, tc := range testCases {
		a, b := mustCards(t, tc.a)[0], mustCards(t, tc.b)[0]

		if got := a.SameSuit(b); got != tc.want {
			t.Errorf("%s.SameSuit(%s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}

		if got := a.CompareSuites(b); got != tc.want {
			t.Errorf("%s.CompareSuites(%s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCardSame(t *testing.T) {
	testCases := []struct {
		a    string
		b    string
		want bool
	}{
		{"2H", "2H", true},
		{"2H", "2C", false},
		{"2H", "3H", false},
		{"AS", "AS", true},
	}

	for _, tc := range testCases {
		a, b := mustCards(t, tc.a)[0], mustCards(t, tc.b)[0]

		if got := a.Same(b); got != tc.want {
			t.Errorf("%s.Same(%s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}

		if got := a.Compare(b); got != tc.want {
			t.Errorf("%s.Compare(%s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCardCompareValuesAndOrdering(t *testing.T) {
	low, high := mustCards(t, "2H")[0], mustCards(t, "AS")[0]

	if got := low.CompareValues(high); got != -1 {
		t.Errorf("CompareValues(2H, AS) = %d, want -1", got)
	}

	if got := high.CompareValues(low); got != 1 {
		t.Errorf("CompareValues(AS, 2H) = %d, want 1", got)
	}

	if got := low.CompareValues(low); got != 0 {
		t.Errorf("CompareValues(2H, 2H) = %d, want 0", got)
	}

	if !low.LessThan(high) {
		t.Error("LessThan(2H, AS) = false, want true")
	}

	if !high.GreaterThan(low) {
		t.Error("GreaterThan(AS, 2H) = false, want true")
	}
}

func TestSuitsValuesAndAliases(t *testing.T) {
	if len(Suits) != SuitsNum {
		t.Errorf("len(Suits) = %d, want %d", len(Suits), SuitsNum)
	}

	if len(CardValues) != ValuesNum {
		t.Errorf("len(CardValues) = %d, want %d", len(CardValues), ValuesNum)
	}

	want := map[Suit]bool{Hearts: true, Diamonds: true, Spades: true, Clubs: true}
	for _, s := range Suits {
		if !want[s] {
			t.Errorf("Suits contains an unexpected suit %q", s)
		}
	}

	if Crosses != Clubs {
		t.Errorf("Crosses = %q, want %q", Crosses, Clubs)
	}

	if got := Suit(Clubs).Color(); got != Black {
		t.Errorf("Clubs.Color() = %d, want %d", got, Black)
	}

	if got := Hearts.Color(); got != Red {
		t.Errorf("Hearts.Color() = %d, want %d", got, Red)
	}
}

func TestSuitFromString(t *testing.T) {
	testCases := []struct {
		input string
		want  Suit
	}{
		{"10H", Hearts},
		{"2C", Clubs},
		{"AS", Spades},
		{"JD", Diamonds},
		{"nonsense", ""},
	}

	for _, tc := range testCases {
		if got := SuitFromString(tc.input); got != tc.want {
			t.Errorf("SuitFromString(%q) = %q, want %q", tc.input, got, tc.want)
		}

		if got := SuiteFromString(tc.input); got != tc.want {
			t.Errorf("SuiteFromString(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

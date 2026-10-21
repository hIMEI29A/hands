package models

import (
	"fmt"
	"testing"
)

func TestParseCardAllValidCards(t *testing.T) {
	for _, suite := range Suites {
		for _, value := range CardValues {
			s := value.String() + string(suite)

			t.Run(s, func(t *testing.T) {
				c, err := ParseCard(s)
				if err != nil {
					t.Fatalf("ParseCard(%q) returned error: %v", s, err)
				}

				if c.Value != value {
					t.Errorf("ParseCard(%q).Value = %v, want %v", s, c.Value, value)
				}

				if c.Suite == nil {
					t.Fatalf("ParseCard(%q).Suite is nil", s)
				}

				if c.Suite.Suite != suite {
					t.Errorf("ParseCard(%q).Suite.Suite = %q, want %q", s, c.Suite.Suite, suite)
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
		suite Suite
	}{
		{"10H", Ten, Hearts},
		{"2C", Two, Crosses},
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

			if c.Suite.Suite != tc.suite {
				t.Errorf("ParseCard(%q).Suite.Suite = %q, want %q", tc.input, c.Suite.Suite, tc.suite)
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

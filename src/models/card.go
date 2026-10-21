package models

import (
	"errors"
	"fmt"
	"hands/src/helpers"
	"regexp"
)

type CardValue int

func cardValueFromString(s string) CardValue {
	cv := Two

	if v, ok := picCardsValues[s]; ok {
		return v
	}

	for i, v := range cardValuesStrings {
		if v == s {
			cv = CardValue(i + 2)
		}
	}

	return cv
}

var cardValuesStrings = []string{
	"2",
	"3",
	"4",
	"5",
	"6",
	"7",
	"8",
	"9",
	"10",
	"J",
	"Q",
	"K",
	"A",
}

var picCardsValues = map[string]CardValue{
	"J": Jack,
	"Q": Queen,
	"K": King,
	"A": Ace,
}

var (
	suiteFromStringPatten      = regexp.MustCompile(`[HDSC]$`)
	cardValueFromStringPattern = regexp.MustCompile(`^\d{1,2}|[JQKA]{1}`)
)

const (
	Two CardValue = iota + 2
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

var CardValues = []CardValue{
	Two,
	Three,
	Four,
	Five,
	Six,
	Seven,
	Eight,
	Nine,
	Ten,
	Jack,
	Queen,
	King,
	Ace,
}

// String returns the string representation of the card value ("2".."10", "J", "Q", "K", "A").
// Values outside Two..Ace never panic and are rendered as "CardValue(N)".
func (cv CardValue) String() string {
	if cv < Two || cv > Ace {
		return fmt.Sprintf("CardValue(%d)", int(cv))
	}

	return cardValuesStrings[cv-2]
}

func NewCardValue(num int) CardValue {
	return CardValue(num + 2)
}

func NewStringSliceFromCards(cards []*Card) []string {
	ss := make([]string, len(cards))

	for i, c := range cards {
		ss[i] = c.String()
	}

	return ss
}

type SuiteColor int8

const (
	Red SuiteColor = iota
	Black
)

type Suite string

func (s Suite) Color() SuiteColor {
	if s == Hearts || s == Diamonds {
		return Red
	}

	return Black
}

// SuiteFromString extracts the suit character from a card string.
// It performs no validation and returns an empty Suite when no suit is found.
//
// Deprecated: use ParseCard instead, which validates the whole card string strictly.
func SuiteFromString(card string) Suite {
	return Suite(suiteFromStringPatten.FindString(card))
}

// CardValueFromString extracts the card value from a card string.
// It performs no validation and silently defaults to Two on unknown values.
//
// Deprecated: use ParseCard instead, which validates the whole card string strictly.
func CardValueFromString(card string) CardValue {
	return cardValueFromString(cardValueFromStringPattern.FindString(card))
}

const (
	SuitesNum = 4
	ValuesNum = 13
)

const asciiUpper = 32 // difference between lowercase and uppercase ASCII letters

func asciiUppercase(s string) (string, error) {
	b := []byte(s)

	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - asciiUpper
		} else if c < 0x20 || c > 0x7e {
			return "", fmt.Errorf("invalid byte %q in card string", c)
		}
	}

	return string(b), nil
}

// parseCardValue strictly parses a card value string ("2".."10", "J", "Q", "K", "A").
func parseCardValue(s string) (CardValue, error) {
	if i, ok := picCardsValues[s]; ok {
		return i, nil
	}

	for i, v := range cardValuesStrings {
		if v == s {
			return CardValue(i + 2), nil
		}
	}

	return 0, fmt.Errorf("invalid card value %q: want one of 2..10, J, Q, K, A", s)
}

// parseCardSuite strictly parses a card suit character (one of H, D, S, C).
func parseCardSuite(s string) (Suite, error) {
	for _, suite := range Suites {
		if Suite(s) == suite {
			return suite, nil
		}
	}

	return "", fmt.Errorf("invalid card suit %q: want one of H, D, S, C", s)
}

// ParseCard parses a card string strictly and returns the matching Card, or an error
// describing what was wrong with the input. The format is concatenated value+suit with
// no separator: value is 2..10, J, Q, K, A; suit is one of H, D, S, C (e.g. "10H", "2C", "AS").
// Lowercase letters are accepted and normalized to uppercase; anything else (empty
// string, unknown values like "1H" or "11H", unknown suits, trailing junk like "AHx")
// is rejected with an error.
func ParseCard(s string) (*Card, error) {
	if s == "" {
		return nil, errors.New("card string is empty: want value+suit, e.g. \"10H\"")
	}

	upper, err := asciiUppercase(s)
	if err != nil {
		return nil, fmt.Errorf("invalid card string %q: %w", s, err)
	}

	valueStr, suiteStr := upper[:len(upper)-1], upper[len(upper)-1:]

	value, err := parseCardValue(valueStr)
	if err != nil {
		return nil, fmt.Errorf("invalid card string %q: %w", s, err)
	}

	suite, err := parseCardSuite(suiteStr)
	if err != nil {
		return nil, fmt.Errorf("invalid card string %q: %w", s, err)
	}

	return &Card{
		Value: value,
		Suite: &CardSuite{
			Color: suite.Color(),
			Suite: suite,
		},
	}, nil
}

const (
	Hearts   Suite = "H"
	Diamonds       = "D"
	Spades         = "S"
	Crosses        = "C"
)

var Suites = []Suite{
	"H",
	"D",
	"S",
	"C",
}

type CardSuite struct {
	Color SuiteColor
	Suite Suite
}

func (same *CardSuite) Compare(other *CardSuite) bool {
	return same.Suite == other.Suite
}

type Card struct {
	Value CardValue
	Suite *CardSuite
}

// NewCardFromString builds a Card from a card string ("10H", "2C", "AS").
// It panics with a descriptive message when the string is not a valid card.
//
// Deprecated: use ParseCard instead, which returns an error instead of panicking.
func NewCardFromString(card string) *Card {
	c, err := ParseCard(card)
	if err != nil {
		panic(fmt.Sprintf("NewCardFromString: %v", err))
	}

	return c
}

func NewRandomCard() *Card {
	suiteNum := helpers.GenerateRandomNumInRange(SuitesNum)
	valueNum := helpers.GenerateRandomNumInRange(ValuesNum)

	suite := Suites[suiteNum]

	c := &Card{
		Value: NewCardValue(valueNum),
		Suite: &CardSuite{
			Color: suite.Color(),
			Suite: suite,
		},
	}

	return c
}

func (same *Card) String() string {
	return fmt.Sprintf("%s%s", same.Value.String(), same.Suite.Suite)
}

func (same *Card) CompareValues(other *Card) int {
	n := 0
	if same.Value == other.Value {
		return n
	}

	switch same.Value < other.Value {
	case true:
		n--
	case false:
		n++
	}

	return n
}

func (same *Card) EqualTo(other *Card) bool {
	return same.CompareValues(other) == 0
}

func (same *Card) LessThan(other *Card) bool {
	return same.CompareValues(other) < 0
}

func (same *Card) GreaterThan(other *Card) bool {
	return same.CompareValues(other) > 0
}

func (same *Card) CompareSuites(other *Card) bool {
	return same.Suite.Compare(other.Suite)
}

func (same *Card) Compare(other *Card) bool {
	return same.CompareValues(other) == 0 && same.CompareSuites(other)
}

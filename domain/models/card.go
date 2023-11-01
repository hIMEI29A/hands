package models

import (
	"fmt"
	"hands/domain/helpers"
	"regexp"
)

type CardValue int

func cardValueFromString(s string) CardValue {
	cv := Two

	if v, ok := picCardsValues[s]; ok {
		return v
	}

	for i, v := range cardValues {
		if v == s {
			cv = CardValue(i + 2)
		}
	}

	return cv
}

var cardValues = []string{
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

func (cv CardValue) String() string {
	return cardValues[cv-2]
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

func SuiteFromString(card string) Suite {
	return Suite(suiteFromStringPatten.FindString(card))
}

func CardValueFromString(card string) CardValue {
	return cardValueFromString(cardValueFromStringPattern.FindString(card))
}

const (
	SuitesNum = 4
	ValuesNum = 13
)

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

func NewCardFromString(card string) *Card {
	suite := SuiteFromString(card)
	value := CardValueFromString(card)

	return &Card{
		Value: value,
		Suite: &CardSuite{
			Color: suite.Color(),
			Suite: suite,
		},
	}
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

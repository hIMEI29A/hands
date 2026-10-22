package tournament

import (
	"testing"

	"hands/src/engine"
)

func validRules() *TournamentRules {
	return &TournamentRules{
		StartingStack: 100,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 2},
		},
		Payouts: []Payout{
			{Place: 1, Amount: 10},
		},
	}
}

func mustNewTournament(t *testing.T, rules *TournamentRules) *Tournament {
	t.Helper()

	tour, err := NewTournament("sng", "tag", nil, rules)
	if err != nil {
		t.Fatalf("NewTournament: %v", err)
	}

	return tour
}

func registerPlayers(t *testing.T, tour *Tournament, names ...string) []*engine.Player {
	t.Helper()

	players := make([]*engine.Player, 0, len(names))

	for _, name := range names {
		player, err := tour.Register(name)
		if err != nil {
			t.Fatalf("Register(%q): %v", name, err)
		}

		players = append(players, player)
	}

	return players
}

func tournamentChips(t *testing.T, tour *Tournament) engine.Chips {
	t.Helper()

	var total engine.Chips

	for _, standing := range tour.Standings() {
		total += standing.Chips
	}

	return total
}

func foldToShowdown(t *testing.T, tour *Tournament) {
	t.Helper()

	for step := 0; step < 20; step++ {
		table := tour.Table()

		switch table.Phase() {
		case engine.PhaseShowdown:
			return
		case engine.PhaseBetting:
			actor, err := table.CurrentActor()
			if err != nil {
				t.Fatalf("CurrentActor: %v", err)
			}

			if _, err := tour.Act(engine.Action{PlayerID: actor.ID, Type: engine.ActionFold}); err != nil {
				t.Fatalf("fold: %v", err)
			}
		case engine.PhaseStreetComplete:
			if err := tour.NextStreet(); err != nil {
				t.Fatalf("NextStreet: %v", err)
			}
		default:
			t.Fatalf("unexpected phase %s", table.Phase())
		}
	}

	t.Fatal("hand did not reach showdown")
}

func playFoldHand(t *testing.T, tour *Tournament) {
	t.Helper()

	if err := tour.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	foldToShowdown(t, tour)

	if _, err := tour.Showdown(); err != nil {
		t.Fatalf("Showdown: %v", err)
	}
}

func checkCallToShowdown(t *testing.T, tour *Tournament) {
	t.Helper()

	for step := 0; step < 100; step++ {
		table := tour.Table()

		switch table.Phase() {
		case engine.PhaseShowdown:
			return
		case engine.PhaseBetting:
			actor, err := table.CurrentActor()
			if err != nil {
				t.Fatalf("CurrentActor: %v", err)
			}

			toCall, err := table.ToCall(actor.ID)
			if err != nil {
				t.Fatalf("ToCall(%s): %v", actor.ID, err)
			}

			action := engine.Action{PlayerID: actor.ID, Type: engine.ActionCheck}
			if toCall > 0 {
				action.Type = engine.ActionCall
			}

			if _, err := tour.Act(action); err != nil {
				t.Fatalf("%s by %s: %v", action.Type, actor.ID, err)
			}
		case engine.PhaseStreetComplete:
			if err := tour.NextStreet(); err != nil {
				t.Fatalf("NextStreet: %v", err)
			}
		default:
			t.Fatalf("unexpected phase %s", table.Phase())
		}
	}

	t.Fatal("hand did not reach showdown")
}

func TestNewTournamentNormalizesRules(t *testing.T) {
	rules := validRules()
	rules.MinPlayers = 0
	rules.MaxPlayers = 0
	rules.Payouts = []Payout{
		{Place: 2, Amount: 5},
		{Place: 1, Amount: 10},
	}

	tour := mustNewTournament(t, rules)
	got := tour.Rules()

	if got.MinPlayers != 2 {
		t.Fatalf("MinPlayers = %d, want 2", got.MinPlayers)
	}

	if got.MaxPlayers != 9 {
		t.Fatalf("MaxPlayers = %d, want 9", got.MaxPlayers)
	}

	if len(got.Payouts) != 2 || got.Payouts[0].Place != 1 || got.Payouts[1].Place != 2 {
		t.Fatalf("Payouts = %+v, want places sorted 1,2", got.Payouts)
	}

	if got.Payouts[0].Amount != 10 || got.Payouts[1].Amount != 5 {
		t.Fatalf("Payouts amounts = %+v, want 10 then 5", got.Payouts)
	}

	// Mutating the caller's rules must not change the tournament copy.
	rules.Levels[0].SmallBlind = 99
	rules.Payouts[0].Amount = 99

	got = tour.Rules()
	if got.Levels[0].SmallBlind != 1 {
		t.Fatalf("Levels[0].SmallBlind = %d after caller mutation, want 1", got.Levels[0].SmallBlind)
	}

	if got.Payouts[1].Amount != 5 {
		t.Fatalf("Payouts[1].Amount = %d after caller mutation, want 5", got.Payouts[1].Amount)
	}
}

func TestNewTournamentRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name  string
		rules func() *TournamentRules
	}{
		{
			name:  "nil rules",
			rules: func() *TournamentRules { return nil },
		},
		{
			name: "zero starting stack",
			rules: func() *TournamentRules {
				r := validRules()
				r.StartingStack = 0
				return r
			},
		},
		{
			name: "min players below two",
			rules: func() *TournamentRules {
				r := validRules()
				r.MinPlayers = 1
				return r
			},
		},
		{
			name: "max players below min players",
			rules: func() *TournamentRules {
				r := validRules()
				r.MinPlayers = 5
				r.MaxPlayers = 3
				return r
			},
		},
		{
			name: "no levels",
			rules: func() *TournamentRules {
				r := validRules()
				r.Levels = nil
				return r
			},
		},
		{
			name: "zero hands on level",
			rules: func() *TournamentRules {
				r := validRules()
				r.Levels[0].Hands = 0
				return r
			},
		},
		{
			name: "bad ante mode",
			rules: func() *TournamentRules {
				r := validRules()
				r.Levels[0].AnteMode = engine.AnteMode("bogus")
				return r
			},
		},
		{
			name: "no payouts",
			rules: func() *TournamentRules {
				r := validRules()
				r.Payouts = nil
				return r
			},
		},
		{
			name: "zero payout amount",
			rules: func() *TournamentRules {
				r := validRules()
				r.Payouts[0].Amount = 0
				return r
			},
		},
		{
			name: "non-contiguous payouts",
			rules: func() *TournamentRules {
				r := validRules()
				r.Payouts = []Payout{
					{Place: 1, Amount: 10},
					{Place: 3, Amount: 5},
				}
				return r
			},
		},
		{
			name: "duplicate payout places",
			rules: func() *TournamentRules {
				r := validRules()
				r.Payouts = []Payout{
					{Place: 1, Amount: 10},
					{Place: 2, Amount: 5},
					{Place: 2, Amount: 6},
				}
				return r
			},
		},
		{
			name: "payout places exceed max players",
			rules: func() *TournamentRules {
				r := validRules()
				r.MaxPlayers = 2
				r.Payouts = []Payout{
					{Place: 1, Amount: 10},
					{Place: 2, Amount: 5},
					{Place: 3, Amount: 1},
				}
				return r
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewTournament("bad", "tag", nil, tt.rules()); err == nil {
				t.Fatal("NewTournament accepted invalid rules")
			}
		})
	}
}

func TestRegisterEnforcesLimitsAndState(t *testing.T) {
	rules := validRules()
	rules.MinPlayers = 2
	rules.MaxPlayers = 2

	tour := mustNewTournament(t, rules)

	if _, err := tour.Register("alice"); err != nil {
		t.Fatalf("Register(alice): %v", err)
	}

	if _, err := tour.Register("bob"); err != nil {
		t.Fatalf("Register(bob): %v", err)
	}

	if _, err := tour.Register("carol"); err == nil {
		t.Fatal("Register accepted more than MaxPlayers players")
	}

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, err := tour.Register("late"); err == nil {
		t.Fatal("Register accepted a player after Start")
	}

	if tour.State() != StateRunning {
		t.Fatalf("State = %s, want %s", tour.State(), StateRunning)
	}
}

type fixedIDMaker struct {
	id string
}

func (m fixedIDMaker) MakeID() string {
	return m.id
}

func TestRegisterRejectsDuplicateIDs(t *testing.T) {
	tour, err := NewTournament("sng", "tag", fixedIDMaker{id: "same"}, validRules())
	if err != nil {
		t.Fatalf("NewTournament: %v", err)
	}

	if _, err := tour.Register("alice"); err != nil {
		t.Fatalf("Register(alice): %v", err)
	}

	if _, err := tour.Register("bob"); err == nil {
		t.Fatal("Register accepted a duplicate player ID")
	}
}

func TestStartRequiresMinPlayers(t *testing.T) {
	rules := validRules()
	rules.MinPlayers = 3

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob")

	if err := tour.Start(); err == nil {
		t.Fatal("Start accepted fewer than MinPlayers players")
	}

	if tour.State() != StateRegistering {
		t.Fatalf("State = %s, want %s", tour.State(), StateRegistering)
	}

	registerPlayers(t, tour, "carol")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start after enough players: %v", err)
	}
}

func TestStartRequiresPayoutPlaces(t *testing.T) {
	rules := validRules()
	rules.MaxPlayers = 5
	rules.Payouts = []Payout{
		{Place: 1, Amount: 10},
		{Place: 2, Amount: 5},
		{Place: 3, Amount: 1},
	}

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob")

	if err := tour.Start(); err == nil {
		t.Fatal("Start accepted fewer players than payout places")
	}

	registerPlayers(t, tour, "carol")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start after enough players: %v", err)
	}
}

func TestTableNilBeforeStart(t *testing.T) {
	tour := mustNewTournament(t, validRules())

	if tour.Table() != nil {
		t.Fatal("Table() is not nil before Start")
	}

	registerPlayers(t, tour, "alice", "bob")

	if tour.Table() != nil {
		t.Fatal("Table() is not nil after registration")
	}

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	table := tour.Table()
	if table == nil {
		t.Fatal("Table() is nil after Start")
	}

	if table.Type != engine.TournamentTableType {
		t.Fatalf("Table.Type = %s, want %s", table.Type, engine.TournamentTableType)
	}

	if table.SmallBlind() != 1 || table.BigBlind() != 2 {
		t.Fatalf("blinds = %d/%d, want 1/2", table.SmallBlind(), table.BigBlind())
	}
}

func TestLevelProgression(t *testing.T) {
	rules := &TournamentRules{
		StartingStack: 1000,
		MinPlayers:    3,
		MaxPlayers:    3,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 2},
			{SmallBlind: 2, BigBlind: 4, Hands: 2},
		},
		Payouts: []Payout{
			{Place: 1, Amount: 10},
		},
	}

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob", "carol")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if tour.Level() != 0 || tour.HandsInLevel() != 0 || tour.HandsPlayed() != 0 {
		t.Fatalf(
			"initial counters level=%d handsInLevel=%d handsPlayed=%d, want 0/0/0",
			tour.Level(),
			tour.HandsInLevel(),
			tour.HandsPlayed(),
		)
	}

	playFoldHand(t, tour)

	if tour.Level() != 0 || tour.HandsInLevel() != 1 || tour.HandsPlayed() != 1 {
		t.Fatalf(
			"after one hand level=%d handsInLevel=%d handsPlayed=%d, want 0/1/1",
			tour.Level(),
			tour.HandsInLevel(),
			tour.HandsPlayed(),
		)
	}

	playFoldHand(t, tour)

	if tour.Level() != 1 || tour.HandsInLevel() != 0 || tour.HandsPlayed() != 2 {
		t.Fatalf(
			"after two hands level=%d handsInLevel=%d handsPlayed=%d, want 1/0/2",
			tour.Level(),
			tour.HandsInLevel(),
			tour.HandsPlayed(),
		)
	}

	if err := tour.StartHand(); err != nil {
		t.Fatalf("StartHand on level 2: %v", err)
	}

	if tour.Table().SmallBlind() != 2 || tour.Table().BigBlind() != 4 {
		t.Fatalf("level 2 blinds = %d/%d, want 2/4", tour.Table().SmallBlind(), tour.Table().BigBlind())
	}

	if err := tour.StartHand(); err == nil {
		t.Fatal("StartHand accepted a second hand while one is in progress")
	}

	foldToShowdown(t, tour)

	if _, err := tour.Showdown(); err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if tour.Level() != 1 || tour.HandsInLevel() != 1 {
		t.Fatalf("after third hand level=%d handsInLevel=%d, want 1/1", tour.Level(), tour.HandsInLevel())
	}

	// The last level repeats: its hand counter keeps growing and the level
	// does not advance.
	playFoldHand(t, tour)

	if tour.Level() != 1 || tour.HandsInLevel() != 2 {
		t.Fatalf("after fourth hand level=%d handsInLevel=%d, want 1/2", tour.Level(), tour.HandsInLevel())
	}

	if err := tour.StartHand(); err != nil {
		t.Fatalf("StartHand on the repeating last level: %v", err)
	}

	if tour.Level() != 1 || tour.HandsInLevel() != 2 {
		t.Fatalf("StartHand changed last level counters to %d/%d, want 1/2", tour.Level(), tour.HandsInLevel())
	}

	foldToShowdown(t, tour)

	if _, err := tour.Showdown(); err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if tour.Level() != 1 || tour.HandsInLevel() != 3 {
		t.Fatalf("after fifth hand level=%d handsInLevel=%d, want 1/3", tour.Level(), tour.HandsInLevel())
	}
}

func TestAssignBustedPlaces(t *testing.T) {
	tests := []struct {
		name            string
		stacks          []engine.Chips
		remainingBefore int
		want            []int
	}{
		{
			name:            "larger stack gets better place",
			stacks:          []engine.Chips{50, 100},
			remainingBefore: 5,
			want:            []int{5, 4},
		},
		{
			name:            "equal stacks share a place",
			stacks:          []engine.Chips{50, 50},
			remainingBefore: 5,
			want:            []int{4, 4},
		},
		{
			name:            "equal group consumes following places",
			stacks:          []engine.Chips{50, 100, 50},
			remainingBefore: 5,
			want:            []int{4, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			busted := make([]*entry, len(tt.stacks))

			for i, stack := range tt.stacks {
				busted[i] = &entry{order: i, startStack: stack}
			}

			assignBustedPlaces(busted, tt.remainingBefore)

			got := make([]int, len(busted))

			for _, e := range busted {
				if !e.eliminated {
					t.Fatalf("entry %d was not marked eliminated", e.order)
				}

				got[e.order] = e.place
			}

			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("places = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func testEntry(id string, order, place int) *entry {
	return &entry{
		player: &engine.Player{ID: id},
		name:   id,
		order:  order,
		place:  place,
	}
}

func TestSplitPrizesTiedPlace(t *testing.T) {
	payouts := []Payout{
		{Place: 1, Amount: 100},
		{Place: 2, Amount: 50},
		{Place: 3, Amount: 30},
		{Place: 4, Amount: 20},
		{Place: 5, Amount: 10},
	}

	entries := []*entry{
		testEntry("a", 0, 1),
		testEntry("b", 1, 2),
		testEntry("c", 2, 3),
		testEntry("d", 3, 4),
		testEntry("e", 4, 4),
	}

	prizes := splitPrizes(entries, payouts, nil)

	if prizes[entries[0]] != 100 {
		t.Fatalf("place 1 prize = %d, want 100", prizes[entries[0]])
	}

	if prizes[entries[1]] != 50 {
		t.Fatalf("place 2 prize = %d, want 50", prizes[entries[1]])
	}

	if prizes[entries[2]] != 30 {
		t.Fatalf("place 3 prize = %d, want 30", prizes[entries[2]])
	}

	if prizes[entries[3]] != 15 || prizes[entries[4]] != 15 {
		t.Fatalf("tied place 4 prizes = %d/%d, want 15/15", prizes[entries[3]], prizes[entries[4]])
	}
}

func TestSplitPrizesRemainderUsesSeatOrder(t *testing.T) {
	payouts := []Payout{{Place: 1, Amount: 31}}

	a := testEntry("a", 0, 1)
	b := testEntry("b", 1, 1)

	// b sits first clockwise from the dealer, so it receives the odd unit.
	prizes := splitPrizes([]*entry{a, b}, payouts, map[string]int{"b": 0, "a": 1})

	if prizes[b] != 16 || prizes[a] != 15 {
		t.Fatalf("seat-ordered prizes = a:%d b:%d, want a:15 b:16", prizes[a], prizes[b])
	}

	// Without a table the registration order decides.
	prizes = splitPrizes([]*entry{a, b}, payouts, nil)

	if prizes[a] != 16 || prizes[b] != 15 {
		t.Fatalf("registration-ordered prizes = a:%d b:%d, want a:16 b:15", prizes[a], prizes[b])
	}
}

func TestFullTournament(t *testing.T) {
	const (
		startingStack = engine.Chips(10)
		playerCount   = 3
		totalChips    = startingStack * playerCount
		payoutTotal   = 40
	)

	rules := &TournamentRules{
		StartingStack: startingStack,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 3},
			{SmallBlind: 2, BigBlind: 4, Hands: 3},
			{SmallBlind: 5, BigBlind: 10, Hands: 3},
		},
		Payouts: []Payout{
			{Place: 1, Amount: 30},
			{Place: 2, Amount: 10},
		},
	}

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob", "carol")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	hands := 0

	for hands < 100 && tour.State() == StateRunning {
		if err := tour.StartHand(); err != nil {
			t.Fatalf("StartHand on hand %d: %v", hands+1, err)
		}

		checkCallToShowdown(t, tour)

		if _, err := tour.Showdown(); err != nil {
			t.Fatalf("Showdown on hand %d: %v", hands+1, err)
		}

		hands++

		if got := tournamentChips(t, tour); got != totalChips {
			t.Fatalf("after hand %d total chips = %d, want %d", hands, got, totalChips)
		}
	}

	if tour.State() != StateFinished {
		t.Fatalf("State = %s after %d hands, want %s", tour.State(), hands, StateFinished)
	}

	standings := tour.Standings()
	if len(standings) != playerCount {
		t.Fatalf("len(Standings()) = %d, want %d", len(standings), playerCount)
	}

	places := make(map[int]int, playerCount)
	survivors := 0
	prizeSum := 0

	for _, standing := range standings {
		if standing.Place < 1 || standing.Place > playerCount {
			t.Fatalf("standing %+v has an invalid place", standing)
		}

		places[standing.Place]++

		if !standing.Eliminated {
			survivors++
		}

		prizeSum += standing.Prize
	}

	if places[1] != 1 {
		t.Fatalf("place counts = %v, want exactly one place-1 winner", places)
	}

	if survivors != 1 {
		t.Fatalf("survivors = %d, want 1", survivors)
	}

	if prizeSum != payoutTotal {
		t.Fatalf("total prizes = %d, want %d", prizeSum, payoutTotal)
	}

	winner := standings[0]
	if winner.Place != 1 || winner.Chips != totalChips || winner.Prize != 30 {
		t.Fatalf("winner standing = %+v, want place 1, %d chips, prize 30", winner, totalChips)
	}

	// No hand or action is possible once the tournament is finished.
	if _, err := tour.Act(engine.Action{PlayerID: winner.PlayerID, Type: engine.ActionFold}); err == nil {
		t.Fatal("Act succeeded after the tournament finished")
	}

	if err := tour.NextStreet(); err == nil {
		t.Fatal("NextStreet succeeded after the tournament finished")
	}

	if err := tour.StartHand(); err == nil {
		t.Fatal("StartHand succeeded after the tournament finished")
	}

	if _, err := tour.Showdown(); err == nil {
		t.Fatal("Showdown succeeded after the tournament finished")
	}
}

func TestStandingsOrderBeforeFinish(t *testing.T) {
	rules := validRules()
	rules.MinPlayers = 3
	rules.MaxPlayers = 3

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob", "carol")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	standings := tour.Standings()
	if len(standings) != 3 {
		t.Fatalf("len(Standings()) = %d, want 3", len(standings))
	}

	for _, standing := range standings {
		if standing.Place != 0 {
			t.Fatalf("active standing %+v has a non-zero place", standing)
		}

		if standing.Eliminated {
			t.Fatalf("active standing %+v is marked eliminated", standing)
		}
	}
}

func TestDuplicatePlayerNamesGetDistinctIDs(t *testing.T) {
	tour := mustNewTournament(t, validRules())

	first := registerPlayers(t, tour, "alice")
	second := registerPlayers(t, tour, "alice")

	if first[0].ID == second[0].ID {
		t.Fatal("two registrations received the same player ID")
	}

	if first[0].GetCurrentChipsAmount() != 100 || second[0].GetCurrentChipsAmount() != 100 {
		t.Fatalf(
			"starting stacks = %d/%d, want 100/100",
			first[0].GetCurrentChipsAmount(),
			second[0].GetCurrentChipsAmount(),
		)
	}
}

func TestRulesReturnsCopy(t *testing.T) {
	rules := validRules()
	rules.Payouts = []Payout{{Place: 1, Amount: 10}}

	tour := mustNewTournament(t, rules)

	got := tour.Rules()
	got.Levels[0].SmallBlind = 42
	got.Payouts[0].Amount = 42

	again := tour.Rules()
	if again.Levels[0].SmallBlind != 1 || again.Payouts[0].Amount != 10 {
		t.Fatalf("Rules() shares state with its caller: %+v", again)
	}
}

func TestErrorsIncludeState(t *testing.T) {
	tour := mustNewTournament(t, validRules())

	if err := tour.StartHand(); err == nil {
		t.Fatal("StartHand succeeded before Start")
	}

	if _, err := tour.Showdown(); err == nil {
		t.Fatal("Showdown succeeded before Start")
	}

	if err := tour.NextStreet(); err == nil {
		t.Fatal("NextStreet succeeded before Start")
	}

	if _, err := tour.Act(engine.Action{PlayerID: "missing", Type: engine.ActionFold}); err == nil {
		t.Fatal("Act succeeded before Start")
	}

	if err := tour.Start(); err == nil {
		t.Fatal("Start succeeded with no players")
	}

	registerPlayers(t, tour, "alice", "bob")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := tour.Start(); err == nil {
		t.Fatal("Start succeeded twice")
	}
}

func TestNewTournamentAcceptsAnteLevels(t *testing.T) {
	rules := validRules()
	rules.Levels = []BlindLevel{
		{SmallBlind: 1, BigBlind: 2, Ante: 1, AnteMode: engine.AnteBigBlind, Hands: 3},
	}

	tour := mustNewTournament(t, rules)

	if got := tour.Rules().Levels[0].AnteMode; got != engine.AnteBigBlind {
		t.Fatalf("AnteMode = %q, want %q", got, engine.AnteBigBlind)
	}

	if got := tour.Rules().Levels[0].Ante; got != 1 {
		t.Fatalf("Ante = %d, want 1", got)
	}
}

func TestStartRequiresMinimumAfterNormalization(t *testing.T) {
	rules := validRules()
	rules.MinPlayers = 0

	tour := mustNewTournament(t, rules)

	if tour.Rules().MinPlayers != 2 {
		t.Fatalf("MinPlayers = %d, want 2", tour.Rules().MinPlayers)
	}

	if err := tour.Start(); err == nil {
		t.Fatal("Start succeeded with no players")
	}

	registerPlayers(t, tour, "alice")

	if err := tour.Start(); err == nil {
		t.Fatal("Start succeeded with one player")
	}
}

func TestPlaceAssignmentWithMixedStacks(t *testing.T) {
	busted := []*entry{
		{order: 0, startStack: 100},
		{order: 1, startStack: 50},
		{order: 2, startStack: 50},
	}

	assignBustedPlaces(busted, 5)

	if busted[0].place != 3 {
		t.Fatalf("stack 100 place = %d, want 3", busted[0].place)
	}

	if busted[1].place != 4 || busted[2].place != 4 {
		t.Fatalf("stack 50 places = %d/%d, want 4/4", busted[1].place, busted[2].place)
	}
}

func TestShowdownWithoutTableFails(t *testing.T) {
	tour := &Tournament{state: StateRunning}

	if _, err := tour.Showdown(); err == nil {
		t.Fatal("Showdown succeeded without a table")
	}
}

func TestStateString(t *testing.T) {
	if string(StateRegistering) != "registering" {
		t.Fatal("StateRegistering has an unexpected value")
	}

	if string(StateRunning) != "running" {
		t.Fatal("StateRunning has an unexpected value")
	}

	if string(StateFinished) != "finished" {
		t.Fatal("StateFinished has an unexpected value")
	}
}

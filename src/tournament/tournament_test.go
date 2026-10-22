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

func percentRules() *TournamentRules {
	return &TournamentRules{
		StartingStack: 100,
		BuyIn:         100,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 2},
		},
		PayoutMode: PayoutModePercent,
		PercentPayouts: []PercentPayout{
			{Place: 1, Percent: 60},
			{Place: 2, Percent: 40},
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
		{
			name: "negative buy-in",
			rules: func() *TournamentRules {
				r := validRules()
				r.BuyIn = -1
				return r
			},
		},
		{
			name: "negative fee",
			rules: func() *TournamentRules {
				r := validRules()
				r.Fee = -1
				return r
			},
		},
		{
			name: "negative guarantee",
			rules: func() *TournamentRules {
				r := validRules()
				r.Guarantee = -1
				return r
			},
		},
		{
			name: "unknown payout mode",
			rules: func() *TournamentRules {
				r := validRules()
				r.PayoutMode = PayoutMode("bogus")
				return r
			},
		},
		{
			name: "fixed mode with percent payouts",
			rules: func() *TournamentRules {
				r := validRules()
				r.PercentPayouts = []PercentPayout{{Place: 1, Percent: 100}}
				return r
			},
		},
		{
			name: "percent mode without percent payouts",
			rules: func() *TournamentRules {
				r := validRules()
				r.Payouts = nil
				r.PayoutMode = PayoutModePercent
				r.BuyIn = 100
				return r
			},
		},
		{
			name: "percent mode with fixed payouts",
			rules: func() *TournamentRules {
				r := percentRules()
				r.Payouts = []Payout{{Place: 1, Amount: 10}}
				return r
			},
		},
		{
			name: "percent payout zero",
			rules: func() *TournamentRules {
				r := percentRules()
				r.PercentPayouts[0].Percent = 0
				return r
			},
		},
		{
			name: "percent payout above 100",
			rules: func() *TournamentRules {
				r := percentRules()
				r.PercentPayouts = []PercentPayout{{Place: 1, Percent: 101}}
				return r
			},
		},
		{
			name: "percent payouts do not sum to 100",
			rules: func() *TournamentRules {
				r := percentRules()
				r.PercentPayouts = []PercentPayout{
					{Place: 1, Percent: 50},
					{Place: 2, Percent: 40},
				}
				return r
			},
		},
		{
			name: "percent payouts non-contiguous",
			rules: func() *TournamentRules {
				r := percentRules()
				r.PercentPayouts = []PercentPayout{
					{Place: 1, Percent: 50},
					{Place: 3, Percent: 50},
				}
				return r
			},
		},
		{
			name: "percent payouts duplicate places",
			rules: func() *TournamentRules {
				r := percentRules()
				r.PercentPayouts = []PercentPayout{
					{Place: 1, Percent: 50},
					{Place: 1, Percent: 50},
				}
				return r
			},
		},
		{
			name: "percent payout places exceed max players",
			rules: func() *TournamentRules {
				r := percentRules()
				r.MaxPlayers = 2
				r.PercentPayouts = []PercentPayout{
					{Place: 1, Percent: 50},
					{Place: 2, Percent: 30},
					{Place: 3, Percent: 20},
				}
				return r
			},
		},
		{
			name: "percent mode without accounting",
			rules: func() *TournamentRules {
				r := percentRules()
				r.BuyIn = 0
				return r
			},
		},
		{
			name: "winner-take-all with fixed payouts",
			rules: func() *TournamentRules {
				r := validRules()
				r.PayoutMode = PayoutModeWinnerTakeAll
				r.BuyIn = 100
				return r
			},
		},
		{
			name: "winner-take-all with percent payouts",
			rules: func() *TournamentRules {
				r := validRules()
				r.Payouts = nil
				r.PayoutMode = PayoutModeWinnerTakeAll
				r.BuyIn = 100
				r.PercentPayouts = []PercentPayout{{Place: 1, Percent: 100}}
				return r
			},
		},
		{
			name: "winner-take-all without accounting",
			rules: func() *TournamentRules {
				r := validRules()
				r.Payouts = nil
				r.PayoutMode = PayoutModeWinnerTakeAll
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

func TestFullTournamentPercentPayouts(t *testing.T) {
	const (
		startingStack = engine.Chips(10)
		playerCount   = 3
		buyIn         = 10
		wantPool      = buyIn * playerCount
	)

	rules := &TournamentRules{
		StartingStack: startingStack,
		BuyIn:         buyIn,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 3},
			{SmallBlind: 2, BigBlind: 4, Hands: 3},
			{SmallBlind: 5, BigBlind: 10, Hands: 3},
		},
		PayoutMode: PayoutModePercent,
		PercentPayouts: []PercentPayout{
			{Place: 1, Percent: 50},
			{Place: 2, Percent: 30},
			{Place: 3, Percent: 20},
		},
	}

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob", "carol")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := tour.PrizePool(); got != wantPool {
		t.Fatalf("PrizePool() = %d, want %d", got, wantPool)
	}

	for hands := 0; hands < 100 && tour.State() == StateRunning; hands++ {
		if err := tour.StartHand(); err != nil {
			t.Fatalf("StartHand on hand %d: %v", hands+1, err)
		}

		checkCallToShowdown(t, tour)

		if _, err := tour.Showdown(); err != nil {
			t.Fatalf("Showdown on hand %d: %v", hands+1, err)
		}
	}

	if tour.State() != StateFinished {
		t.Fatalf("State = %s, want %s", tour.State(), StateFinished)
	}

	standings := tour.Standings()
	prizeSum := 0

	for _, standing := range standings {
		prizeSum += standing.Prize
	}

	if prizeSum != wantPool {
		t.Fatalf("total prizes = %d, want %d", prizeSum, wantPool)
	}

	if standings[0].Place != 1 || standings[0].Prize != wantPool/2 {
		t.Fatalf("winner standing = %+v, want place 1 and prize %d", standings[0], wantPool/2)
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

func TestPayoutModeDefaultsToFixed(t *testing.T) {
	tour := mustNewTournament(t, validRules())

	if got := tour.Rules().PayoutMode; got != PayoutModeFixed {
		t.Fatalf("PayoutMode = %q, want %q", got, PayoutModeFixed)
	}
}

func TestPrizePoolTracksRegistrations(t *testing.T) {
	rules := validRules()
	rules.BuyIn = 100
	rules.Fee = 10

	tour := mustNewTournament(t, rules)

	if got := tour.PrizePool(); got != 0 {
		t.Fatalf("PrizePool() = %d before registration, want 0", got)
	}

	registerPlayers(t, tour, "alice")

	if got := tour.PrizePool(); got != 100 {
		t.Fatalf("PrizePool() = %d after one entry, want 100", got)
	}

	registerPlayers(t, tour, "bob", "carol")

	if got := tour.PrizePool(); got != 300 {
		t.Fatalf("PrizePool() = %d after three entries, want 300", got)
	}
}

func TestEntryCostAndFeesCollected(t *testing.T) {
	rules := validRules()
	rules.BuyIn = 100
	rules.Fee = 10

	tour := mustNewTournament(t, rules)

	if got := tour.EntryCost(); got != 110 {
		t.Fatalf("EntryCost() = %d, want 110", got)
	}

	if got := tour.FeesCollected(); got != 0 {
		t.Fatalf("FeesCollected() = %d before registration, want 0", got)
	}

	registerPlayers(t, tour, "alice", "bob", "carol")

	if got := tour.FeesCollected(); got != 30 {
		t.Fatalf("FeesCollected() = %d after three entries, want 30", got)
	}

	if got := tour.PrizePool(); got != 300 {
		t.Fatalf("PrizePool() = %d after three entries, want 300", got)
	}
}

func TestPrizePool(t *testing.T) {
	tests := []struct {
		name      string
		buyIn     int
		fee       int
		guarantee int
		entries   int
		want      int
	}{
		{
			name:    "buy-in per entry ignores the fee",
			buyIn:   100,
			fee:     10,
			entries: 3,
			want:    300,
		},
		{
			name:      "guarantee overrides a smaller pool",
			buyIn:     100,
			guarantee: 500,
			entries:   2,
			want:      500,
		},
		{
			name:      "guarantee without a buy-in",
			guarantee: 500,
			entries:   3,
			want:      500,
		},
		{
			name:      "guarantee before registration",
			guarantee: 500,
			entries:   0,
			want:      500,
		},
		{
			name:      "buy-in beats a smaller guarantee",
			buyIn:     100,
			guarantee: 50,
			entries:   2,
			want:      200,
		},
		{
			name:    "legacy fixed mode has no pool",
			entries: 3,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tour := &Tournament{
				rules: TournamentRules{
					BuyIn:     tt.buyIn,
					Fee:       tt.fee,
					Guarantee: tt.guarantee,
				},
				entries: make([]*entry, tt.entries),
			}

			if got := tour.PrizePool(); got != tt.want {
				t.Fatalf("PrizePool() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPercentPrizeAmounts(t *testing.T) {
	tests := []struct {
		name    string
		pool    int
		payouts []PercentPayout
		want    []int
	}{
		{
			name: "remainder goes to the largest fraction",
			pool: 101,
			payouts: []PercentPayout{
				{Place: 1, Percent: 50},
				{Place: 2, Percent: 30},
				{Place: 3, Percent: 20},
			},
			want: []int{51, 30, 20},
		},
		{
			name: "even split",
			pool: 100,
			payouts: []PercentPayout{
				{Place: 1, Percent: 33},
				{Place: 2, Percent: 33},
				{Place: 3, Percent: 34},
			},
			want: []int{33, 33, 34},
		},
		{
			name: "equal remainders favour the lower place",
			pool: 3,
			payouts: []PercentPayout{
				{Place: 1, Percent: 50},
				{Place: 2, Percent: 50},
			},
			want: []int{2, 1},
		},
		{
			name: "small pool can leave a place at zero",
			pool: 1,
			payouts: []PercentPayout{
				{Place: 1, Percent: 50},
				{Place: 2, Percent: 50},
			},
			want: []int{1, 0},
		},
		{
			name: "whole pool",
			pool: 7,
			payouts: []PercentPayout{
				{Place: 1, Percent: 100},
			},
			want: []int{7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amounts := percentPrizeAmounts(tt.pool, tt.payouts)

			for i, want := range tt.want {
				place := i + 1
				if got := amounts[place]; got != want {
					t.Fatalf("place %d amount = %d, want %d", place, got, want)
				}
			}
		})
	}
}

func TestPercentPrizeAmountsDistributeWholePool(t *testing.T) {
	structures := [][]PercentPayout{
		{
			{Place: 1, Percent: 50},
			{Place: 2, Percent: 30},
			{Place: 3, Percent: 20},
		},
		{
			{Place: 1, Percent: 33},
			{Place: 2, Percent: 33},
			{Place: 3, Percent: 34},
		},
		{
			{Place: 1, Percent: 60},
			{Place: 2, Percent: 40},
		},
		{
			{Place: 1, Percent: 40},
			{Place: 2, Percent: 30},
			{Place: 3, Percent: 20},
			{Place: 4, Percent: 10},
		},
	}

	for _, payouts := range structures {
		for pool := 0; pool <= 200; pool++ {
			amounts := percentPrizeAmounts(pool, payouts)
			total := 0

			for _, payout := range payouts {
				if amounts[payout.Place] < 0 {
					t.Fatalf("pool %d place %d amount = %d, want non-negative", pool, payout.Place, amounts[payout.Place])
				}

				total += amounts[payout.Place]
			}

			if total != pool {
				t.Fatalf("pool %d amounts %+v total = %d, want %d", pool, amounts, total, pool)
			}
		}
	}
}

func TestAwardPrizesByMode(t *testing.T) {
	tests := []struct {
		name   string
		rules  func() *TournamentRules
		places []int
		want   []int
	}{
		{
			name: "legacy fixed payouts",
			rules: func() *TournamentRules {
				r := validRules()
				r.Payouts = []Payout{
					{Place: 1, Amount: 30},
					{Place: 2, Amount: 10},
				}
				return r
			},
			places: []int{1, 2, 3},
			want:   []int{30, 10, 0},
		},
		{
			name: "fixed payouts funded by a buy-in",
			rules: func() *TournamentRules {
				r := validRules()
				r.BuyIn = 100
				r.Payouts = []Payout{
					{Place: 1, Amount: 200},
					{Place: 2, Amount: 50},
				}
				return r
			},
			places: []int{1, 2, 3},
			want:   []int{200, 50, 0},
		},
		{
			name: "percent payouts",
			rules: func() *TournamentRules {
				r := validRules()
				r.BuyIn = 100
				r.Payouts = nil
				r.PayoutMode = PayoutModePercent
				r.PercentPayouts = []PercentPayout{
					{Place: 1, Percent: 50},
					{Place: 2, Percent: 30},
					{Place: 3, Percent: 20},
				}
				return r
			},
			places: []int{1, 2, 3},
			want:   []int{150, 90, 60},
		},
		{
			name: "winner takes all",
			rules: func() *TournamentRules {
				r := validRules()
				r.BuyIn = 100
				r.Payouts = nil
				r.PayoutMode = PayoutModeWinnerTakeAll
				return r
			},
			places: []int{1, 2, 3},
			want:   []int{300, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := normalizeRules(tt.rules())
			if err != nil {
				t.Fatalf("normalizeRules: %v", err)
			}

			entries := make([]*entry, len(tt.places))
			for i, place := range tt.places {
				entries[i] = testEntry(string(rune('a'+i)), i, place)
			}

			tour := &Tournament{rules: normalized, entries: entries}
			tour.awardPrizesLocked()

			for i, want := range tt.want {
				if got := entries[i].prize; got != want {
					t.Fatalf("entry %d prize = %d, want %d", i, got, want)
				}
			}
		})
	}
}

func TestAwardPrizesSplitsPercentTies(t *testing.T) {
	rules := &TournamentRules{
		StartingStack: 100,
		Guarantee:     101,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 2},
		},
		PayoutMode: PayoutModePercent,
		PercentPayouts: []PercentPayout{
			{Place: 1, Percent: 60},
			{Place: 2, Percent: 40},
		},
	}

	normalized, err := normalizeRules(rules)
	if err != nil {
		t.Fatalf("normalizeRules: %v", err)
	}

	a := testEntry("a", 0, 1)
	b := testEntry("b", 1, 1)

	tour := &Tournament{rules: normalized, entries: []*entry{a, b}}
	tour.awardPrizesLocked()

	// Places 1 and 2 are worth 61 and 40 after rounding; the tied players
	// split 101 evenly and the odd unit follows registration order.
	if a.prize != 51 || b.prize != 50 {
		t.Fatalf("tied percent prizes = a:%d b:%d, want a:51 b:50", a.prize, b.prize)
	}
}

func TestAwardPrizesSplitsZeroPrizePlace(t *testing.T) {
	rules := &TournamentRules{
		StartingStack: 100,
		Guarantee:     1,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 2},
		},
		PayoutMode: PayoutModePercent,
		PercentPayouts: []PercentPayout{
			{Place: 1, Percent: 50},
			{Place: 2, Percent: 50},
		},
	}

	normalized, err := normalizeRules(rules)
	if err != nil {
		t.Fatalf("normalizeRules: %v", err)
	}

	winner := testEntry("winner", 0, 1)
	first := testEntry("first", 1, 2)
	second := testEntry("second", 2, 2)

	tour := &Tournament{
		rules:   normalized,
		entries: []*entry{winner, first, second},
	}
	tour.awardPrizesLocked()

	// The whole pool is one unit, so place 2 is worth zero.
	if winner.prize != 1 {
		t.Fatalf("winner prize = %d, want 1", winner.prize)
	}

	if first.prize != 0 || second.prize != 0 {
		t.Fatalf("tied zero-prize places = %d/%d, want 0/0", first.prize, second.prize)
	}
}

func TestRulesCopiesPercentPayouts(t *testing.T) {
	rules := percentRules()
	rules.PercentPayouts = []PercentPayout{
		{Place: 2, Percent: 40},
		{Place: 1, Percent: 60},
	}

	tour := mustNewTournament(t, rules)

	got := tour.Rules()
	if len(got.PercentPayouts) != 2 ||
		got.PercentPayouts[0].Place != 1 ||
		got.PercentPayouts[1].Place != 2 {
		t.Fatalf("PercentPayouts = %+v, want places sorted 1,2", got.PercentPayouts)
	}

	got.PercentPayouts[0].Percent = 1

	again := tour.Rules()
	if again.PercentPayouts[0].Percent != 60 {
		t.Fatalf("Rules() shares PercentPayouts with its caller: %+v", again.PercentPayouts)
	}
}

func TestStartValidatesPayoutFunding(t *testing.T) {
	tests := []struct {
		name    string
		rules   func() *TournamentRules
		players []string
		wantErr bool
	}{
		{
			name: "fixed payouts fit the pool",
			rules: func() *TournamentRules {
				r := validRules()
				r.BuyIn = 100
				r.Payouts = []Payout{
					{Place: 1, Amount: 200},
					{Place: 2, Amount: 50},
				}
				return r
			},
			players: []string{"alice", "bob", "carol"},
		},
		{
			name: "fixed payouts funded by a guarantee",
			rules: func() *TournamentRules {
				r := validRules()
				r.Guarantee = 300
				r.Payouts = []Payout{
					{Place: 1, Amount: 200},
					{Place: 2, Amount: 50},
				}
				return r
			},
			players: []string{"alice", "bob"},
		},
		{
			name: "fixed payouts exceed the pool",
			rules: func() *TournamentRules {
				r := validRules()
				r.BuyIn = 100
				r.Payouts = []Payout{
					{Place: 1, Amount: 200},
					{Place: 2, Amount: 50},
				}
				return r
			},
			players: []string{"alice", "bob"},
			wantErr: true,
		},
		{
			name: "percent places fit the entries",
			rules: func() *TournamentRules {
				r := percentRules()
				r.PercentPayouts = []PercentPayout{
					{Place: 1, Percent: 50},
					{Place: 2, Percent: 30},
					{Place: 3, Percent: 20},
				}
				return r
			},
			players: []string{"alice", "bob", "carol"},
		},
		{
			name: "percent places exceed the entries",
			rules: func() *TournamentRules {
				r := percentRules()
				r.PercentPayouts = []PercentPayout{
					{Place: 1, Percent: 50},
					{Place: 2, Percent: 30},
					{Place: 3, Percent: 20},
				}
				return r
			},
			players: []string{"alice", "bob"},
			wantErr: true,
		},
		{
			name: "winner-take-all starts with two entries",
			rules: func() *TournamentRules {
				r := validRules()
				r.BuyIn = 100
				r.Payouts = nil
				r.PayoutMode = PayoutModeWinnerTakeAll
				return r
			},
			players: []string{"alice", "bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tour := mustNewTournament(t, tt.rules())
			registerPlayers(t, tour, tt.players...)

			err := tour.Start()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Start accepted an unfunded payout")
				}

				if tour.State() != StateRegistering {
					t.Fatalf("State = %s after a rejected Start, want %s", tour.State(), StateRegistering)
				}

				return
			}

			if err != nil {
				t.Fatalf("Start: %v", err)
			}
		})
	}
}

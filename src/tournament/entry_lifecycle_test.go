package tournament

import (
	"strings"
	"testing"

	"hands/src/engine"
)

// lifecycleRules returns a four-player setup with three long levels, so a
// test can bust a player without losing the late registration window.
func lifecycleRules() *TournamentRules {
	return &TournamentRules{
		StartingStack: 8,
		MinPlayers:    2,
		MaxPlayers:    4,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 100},
			{SmallBlind: 2, BigBlind: 4, Hands: 100},
			{SmallBlind: 5, BigBlind: 10, Hands: 100},
		},
		BuyIn:             10,
		Fee:               1,
		LateRegUntilLevel: 3,
		Payouts: []Payout{
			{Place: 1, Amount: 10},
		},
	}
}

// playIsolationHand starts a hand, shoves with the first player to act and
// folds everybody else out until the last player calls all-in. The hand
// therefore always ends with exactly two all-in players, so at most one
// player busts. A split pot simply leaves the stacks roughly unchanged and
// the caller may try again.
func playIsolationHand(t *testing.T, tour *Tournament) {
	t.Helper()

	if err := tour.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	table := tour.Table()

	// With very short stacks the blinds can put every player all-in
	// before anyone acts.
	if table.Phase() == engine.PhaseBetting {
		actor, err := table.CurrentActor()
		if err != nil {
			t.Fatalf("CurrentActor: %v", err)
		}

		stack := actor.GetCurrentChipsAmount()
		toCall, err := table.ToCall(actor.ID)
		if err != nil {
			t.Fatalf("ToCall(%s): %v", actor.ID, err)
		}

		action := engine.Action{
			PlayerID: actor.ID,
			Type:     engine.ActionRaise,
			Amount:   actor.GetStreetBet() + stack,
		}

		if toCall >= stack {
			action = engine.Action{PlayerID: actor.ID, Type: engine.ActionCall}
		}

		if _, err := tour.Act(action); err != nil {
			t.Fatalf("%s by %s: %v", action.Type, actor.ID, err)
		}
	}

	for step := 0; step < 20; step++ {
		table = tour.Table()

		if table.Phase() != engine.PhaseBetting {
			break
		}

		actor, err := table.CurrentActor()
		if err != nil {
			t.Fatalf("CurrentActor: %v", err)
		}

		active := 0
		for _, p := range table.Players {
			if p.Active {
				active++
			}
		}

		if active > 2 {
			if _, err := tour.Act(engine.Action{PlayerID: actor.ID, Type: engine.ActionFold}); err != nil {
				t.Fatalf("fold by %s: %v", actor.ID, err)
			}

			continue
		}

		toCall, err := table.ToCall(actor.ID)
		if err != nil {
			t.Fatalf("ToCall(%s): %v", actor.ID, err)
		}

		call := engine.Action{PlayerID: actor.ID, Type: engine.ActionCheck}
		if toCall > 0 {
			call.Type = engine.ActionCall
		}

		if _, err := tour.Act(call); err != nil {
			t.Fatalf("%s by %s: %v", call.Type, actor.ID, err)
		}
	}

	checkCallToShowdown(t, tour)

	if _, err := tour.Showdown(); err != nil {
		t.Fatalf("Showdown: %v", err)
	}
}

// bustOneRunning plays isolation hands until exactly one entry busts while
// the tournament is still running. The caller must start with at least three
// funded players so the tournament survives the bust.
func bustOneRunning(t *testing.T, tour *Tournament) Standing {
	t.Helper()

	for hand := 0; hand < 50; hand++ {
		if tour.State() != StateRunning {
			t.Fatalf("tournament finished before a player busted")
		}

		before := make(map[string]bool)
		for _, standing := range tour.Standings() {
			if standing.Eliminated {
				before[standing.PlayerID] = true
			}
		}

		playIsolationHand(t, tour)

		for _, standing := range tour.Standings() {
			if standing.Eliminated && !before[standing.PlayerID] {
				if tour.State() != StateRunning {
					t.Fatalf("tournament finished when %s busted; use more players", standing.Name)
				}

				return standing
			}
		}
	}

	t.Fatal("no player busted in 50 hands")
	return Standing{}
}

func standingsByID(tour *Tournament) map[string]Standing {
	byID := make(map[string]Standing)

	for _, standing := range tour.Standings() {
		byID[standing.PlayerID] = standing
	}

	return byID
}

// tableHasPlayer reports whether playerID still holds a table seat.
func tableHasPlayer(tour *Tournament, playerID string) bool {
	for _, player := range tour.Table().Players {
		if player.ID == playerID {
			return true
		}
	}

	return false
}

func tablePlayerIndex(t *testing.T, tour *Tournament, playerID string) int {
	t.Helper()

	table := tour.Table()

	for i, player := range table.Players {
		if player.ID == playerID {
			return i
		}
	}

	t.Fatalf("player %s is not at the table", playerID)
	return -1
}

func TestLateRegistrationBeforeCutoff(t *testing.T) {
	rules := lifecycleRules()
	rules.MaxPlayers = 4

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	carol, err := tour.Register("carol")
	if err != nil {
		t.Fatalf("late Register(carol): %v", err)
	}

	if carol.GetCurrentChipsAmount() != rules.StartingStack {
		t.Fatalf("late registrant chips = %d, want %d", carol.GetCurrentChipsAmount(), rules.StartingStack)
	}

	if got := len(tour.Table().Players); got != 3 {
		t.Fatalf("seated players = %d, want 3", got)
	}

	if got := tour.PrizePool(); got != 30 {
		t.Fatalf("PrizePool() = %d after late registration, want 30", got)
	}

	if got := tour.FeesCollected(); got != 3 {
		t.Fatalf("FeesCollected() = %d after late registration, want 3", got)
	}

	standings := standingsByID(tour)
	late, ok := standings[carol.ID]
	if !ok {
		t.Fatalf("late registrant %s is missing from the standings", carol.ID)
	}

	if late.Name != "carol" || late.Eliminated || late.Place != 0 || late.Chips != rules.StartingStack {
		t.Fatalf("late standing = %+v, want an active carol with %d chips", late, rules.StartingStack)
	}

	if _, err := tour.Register("dave"); err != nil {
		t.Fatalf("second late Register(dave): %v", err)
	}

	if got := tour.PrizePool(); got != 40 {
		t.Fatalf("PrizePool() = %d after two late registrations, want 40", got)
	}

	if _, err := tour.Register("eve"); err == nil {
		t.Fatal("Register accepted a late registrant without a free or busted seat")
	}
}

func TestLateRegistrationClosedAfterCutoff(t *testing.T) {
	rules := &TournamentRules{
		StartingStack: 20,
		MinPlayers:    2,
		MaxPlayers:    5,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 1},
			{SmallBlind: 2, BigBlind: 4, Hands: 1},
			{SmallBlind: 5, BigBlind: 10, Hands: 1},
		},
		BuyIn:             10,
		LateRegUntilLevel: 1,
		Payouts: []Payout{
			{Place: 1, Amount: 10},
		},
	}

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob", "carol")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	playFoldHand(t, tour)

	if tour.Level() != 1 {
		t.Fatalf("Level = %d after the first level, want 1", tour.Level())
	}

	_, err := tour.Register("late")
	if err == nil {
		t.Fatal("Register accepted a late registrant after the cutoff")
	}

	if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Register error = %q, want it to mention closed late registration", err)
	}
}

func TestRegisterRejectedAfterFinish(t *testing.T) {
	tour := mustNewTournament(t, lifecycleRules())
	registerPlayers(t, tour, "alice", "bob", "carol", "dave")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	for tour.State() == StateRunning {
		playIsolationHand(t, tour)
	}

	if _, err := tour.Register("late"); err == nil {
		t.Fatal("Register accepted a player after the tournament finished")
	}
}

func TestLateRegistrationReusesBustedSeat(t *testing.T) {
	rules := lifecycleRules()

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob", "carol", "dave")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	busted := bustOneRunning(t, tour)

	if busted.Place != 4 {
		t.Fatalf("busted place = %d, want 4", busted.Place)
	}

	bustedIndex := tablePlayerIndex(t, tour, busted.PlayerID)

	late, err := tour.Register("late")
	if err != nil {
		t.Fatalf("Register after a bust: %v", err)
	}

	if got := len(tour.Table().Players); got != 4 {
		t.Fatalf("seated players = %d after reusing a busted seat, want 4", got)
	}

	if got := tablePlayerIndex(t, tour, late.ID); got != bustedIndex {
		t.Fatalf("late registrant seat = %d, want the busted seat %d", got, bustedIndex)
	}

	if got := tour.Table().Players[bustedIndex].ID; got != late.ID {
		t.Fatalf("busted seat holds %s, want %s", got, late.ID)
	}

	standings := standingsByID(tour)

	old, ok := standings[busted.PlayerID]
	if !ok {
		t.Fatalf("busted entry %s disappeared from the standings", busted.PlayerID)
	}

	if !old.Eliminated || old.Place != 5 {
		t.Fatalf("busted standing = %+v, want place 5 and eliminated", old)
	}

	fresh, ok := standings[late.ID]
	if !ok {
		t.Fatalf("late registrant %s is missing from the standings", late.ID)
	}

	if fresh.Eliminated || fresh.Place != 0 || fresh.Chips != rules.StartingStack {
		t.Fatalf("late standing = %+v, want an active entry with %d chips", fresh, rules.StartingStack)
	}

	if got := len(tour.Standings()); got != 5 {
		t.Fatalf("standings rows = %d, want 5", got)
	}
}

func TestReentryBasics(t *testing.T) {
	rules := lifecycleRules()
	rules.ReentryAllowed = true

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob", "carol", "dave")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	busted := bustOneRunning(t, tour)
	bustedIndex := tablePlayerIndex(t, tour, busted.PlayerID)

	active := ""
	for _, standing := range tour.Standings() {
		if !standing.Eliminated {
			active = standing.PlayerID

			break
		}
	}

	if _, err := tour.Reenter(active); err == nil {
		t.Fatal("Reenter accepted a player who is still in the tournament")
	}

	if _, err := tour.Reenter("missing"); err == nil {
		t.Fatal("Reenter accepted an unknown player ID")
	}

	player, err := tour.Reenter(busted.PlayerID)
	if err != nil {
		t.Fatalf("Reenter(%s): %v", busted.PlayerID, err)
	}

	if player.ID == busted.PlayerID {
		t.Fatal("re-entry reused the eliminated player ID")
	}

	if player.Name != busted.Name {
		t.Fatalf("re-entry name = %q, want %q", player.Name, busted.Name)
	}

	if player.GetCurrentChipsAmount() != rules.StartingStack {
		t.Fatalf("re-entry chips = %d, want %d", player.GetCurrentChipsAmount(), rules.StartingStack)
	}

	if got := tablePlayerIndex(t, tour, player.ID); got != bustedIndex {
		t.Fatalf("re-entry seat = %d, want the busted seat %d", got, bustedIndex)
	}

	standings := standingsByID(tour)

	old, ok := standings[busted.PlayerID]
	if !ok {
		t.Fatalf("busted entry %s disappeared from the standings", busted.PlayerID)
	}

	if !old.Eliminated || old.Place != 5 {
		t.Fatalf("busted standing = %+v, want place 5 and eliminated", old)
	}

	fresh, ok := standings[player.ID]
	if !ok {
		t.Fatalf("re-entry %s is missing from the standings", player.ID)
	}

	if fresh.Eliminated || fresh.Place != 0 || fresh.Name != busted.Name {
		t.Fatalf("re-entry standing = %+v, want an active entry named %q", fresh, busted.Name)
	}

	if got := len(tour.Standings()); got != 5 {
		t.Fatalf("standings rows = %d, want 5", got)
	}

	if got := tour.PrizePool(); got != 50 {
		t.Fatalf("PrizePool() = %d after re-entry, want 50", got)
	}

	if got := tour.FeesCollected(); got != 5 {
		t.Fatalf("FeesCollected() = %d after re-entry, want 5", got)
	}

	// The new entry belongs to the busted entry's re-entry chain.
	var root *entry

	for _, e := range tour.entries {
		if e.player.ID == player.ID {
			root = e.root
		}
	}

	if root == nil || root.player.ID != busted.PlayerID {
		t.Fatalf("re-entry root = %+v, want the busted entry %s", root, busted.PlayerID)
	}
}

func TestReentryDisabled(t *testing.T) {
	tour := mustNewTournament(t, lifecycleRules())
	registerPlayers(t, tour, "alice", "bob", "carol", "dave")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	busted := bustOneRunning(t, tour)

	_, err := tour.Reenter(busted.PlayerID)
	if err == nil {
		t.Fatal("Reenter succeeded with re-entry disabled")
	}

	if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("Reenter error = %q, want it to mention disabled re-entry", err)
	}
}

func TestReentryAfterWindow(t *testing.T) {
	rules := &TournamentRules{
		StartingStack: 20,
		MinPlayers:    2,
		MaxPlayers:    4,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 1},
			{SmallBlind: 2, BigBlind: 4, Hands: 100},
		},
		LateRegUntilLevel: 1,
		ReentryAllowed:    true,
		BuyIn:             10,
		Payouts: []Payout{
			{Place: 1, Amount: 10},
		},
	}

	tour := mustNewTournament(t, rules)
	players := registerPlayers(t, tour, "alice", "bob", "carol", "dave")

	if _, err := tour.Reenter(players[0].ID); err == nil {
		t.Fatal("Reenter succeeded before the tournament started")
	}

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	playFoldHand(t, tour)

	if tour.Level() != 1 {
		t.Fatalf("Level = %d, want 1", tour.Level())
	}

	_, err := tour.Reenter(players[0].ID)
	if err == nil {
		t.Fatal("Reenter succeeded after the late registration window")
	}

	if !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Reenter error = %q, want it to mention closed late registration", err)
	}
}

func TestReentryRejectsActiveChain(t *testing.T) {
	rules := lifecycleRules()
	rules.ReentryAllowed = true

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob", "carol", "dave")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	busted := bustOneRunning(t, tour)

	first, err := tour.Reenter(busted.PlayerID)
	if err != nil {
		t.Fatalf("first Reenter: %v", err)
	}

	if first.ID == busted.PlayerID {
		t.Fatal("first re-entry reused the eliminated player ID")
	}

	if _, err := tour.Reenter(busted.PlayerID); err == nil {
		t.Fatal("Reenter accepted a second entry while the first re-entry is active")
	} else if !strings.Contains(err.Error(), "active entry") {
		t.Fatalf("Reenter error = %q, want it to mention an active entry", err)
	}
}

func TestReentryMaxLimit(t *testing.T) {
	root := testEntry("root", 0, 0)
	root.eliminated = true
	root.reentries = 2

	tour := &Tournament{
		rules: TournamentRules{
			LateRegUntilLevel: 1,
			ReentryAllowed:    true,
			MaxReentries:      2,
		},
		entries: []*entry{root},
		table:   &engine.Table{},
		state:   StateRunning,
	}

	_, err := tour.Reenter(root.player.ID)
	if err == nil {
		t.Fatal("Reenter exceeded MaxReentries")
	}

	if !strings.Contains(err.Error(), "limit") {
		t.Fatalf("Reenter error = %q, want it to mention the re-entry limit", err)
	}
}

func TestReentryFallsBackToFreeSeat(t *testing.T) {
	rules := lifecycleRules()
	rules.MaxPlayers = 4
	rules.ReentryAllowed = true
	rules.MaxReentries = 2

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob", "carol", "dave")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	first := bustOneRunning(t, tour)

	late, err := tour.Register("late")
	if err != nil {
		t.Fatalf("Register after a bust: %v", err)
	}

	if tableHasPlayer(tour, first.PlayerID) {
		t.Fatalf("eliminated player %s is still at the table after late registration", first.PlayerID)
	}

	second := bustOneRunning(t, tour)
	secondIndex := tablePlayerIndex(t, tour, second.PlayerID)

	// first's old seat was taken by the late registrant, so the re-entry
	// falls back to SeatPlayer and reuses second's busted seat.
	reentered, err := tour.Reenter(first.PlayerID)
	if err != nil {
		t.Fatalf("Reenter(%s): %v", first.PlayerID, err)
	}

	if reentered.ID == first.PlayerID || reentered.ID == late.ID {
		t.Fatal("re-entry did not receive a new player ID")
	}

	if got := len(tour.Table().Players); got != 4 {
		t.Fatalf("seated players = %d after re-entry, want 4", got)
	}

	if got := tablePlayerIndex(t, tour, reentered.ID); got != secondIndex {
		t.Fatalf("re-entry seat = %d, want the busted seat %d", got, secondIndex)
	}

	standings := standingsByID(tour)

	old, ok := standings[first.PlayerID]
	if !ok {
		t.Fatalf("busted entry %s disappeared from the standings", first.PlayerID)
	}

	if !old.Eliminated {
		t.Fatalf("busted standing = %+v, want it to remain eliminated", old)
	}

	fresh, ok := standings[reentered.ID]
	if !ok {
		t.Fatalf("re-entry %s is missing from the standings", reentered.ID)
	}

	if fresh.Eliminated || fresh.Chips != rules.StartingStack {
		t.Fatalf("re-entry standing = %+v, want an active entry with %d chips", fresh, rules.StartingStack)
	}
}

func TestShiftEliminatedPlacesKeepsFinalPlacesUnique(t *testing.T) {
	a := testEntry("a", 0, 0)
	a.startStack = 100

	b := testEntry("b", 1, 0)
	b.startStack = 80

	c := testEntry("c", 2, 0)
	c.startStack = 60

	d := testEntry("d", 3, 0)
	d.startStack = 40

	tour := &Tournament{entries: []*entry{a, b, c, d}}

	assignBustedPlaces([]*entry{d}, 4)

	if d.place != 4 || !d.eliminated {
		t.Fatalf("first busted place = %d eliminated = %v, want 4/true", d.place, d.eliminated)
	}

	tour.shiftEliminatedPlacesLocked()

	if d.place != 5 {
		t.Fatalf("shifted place = %d, want 5", d.place)
	}

	late := testEntry("late", 4, 0)
	late.startStack = 20
	tour.entries = append(tour.entries, late)

	assignBustedPlaces([]*entry{c}, 4)
	assignBustedPlaces([]*entry{b}, 3)
	assignBustedPlaces([]*entry{a}, 2)
	late.place = 1

	want := map[string]int{"a": 2, "b": 3, "c": 4, "d": 5, "late": 1}
	seen := make(map[int]bool, len(tour.entries))

	for _, e := range tour.entries {
		if e.place != want[e.player.ID] {
			t.Fatalf("entry %s place = %d, want %d", e.player.ID, e.place, want[e.player.ID])
		}

		if seen[e.place] {
			t.Fatalf("duplicate final place %d", e.place)
		}

		seen[e.place] = true
	}

	for place := 1; place <= len(tour.entries); place++ {
		if !seen[place] {
			t.Fatalf("missing final place %d", place)
		}
	}
}

func TestAddOnWindow(t *testing.T) {
	rules := &TournamentRules{
		StartingStack: 8,
		MinPlayers:    2,
		MaxPlayers:    4,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 5},
			{SmallBlind: 2, BigBlind: 4, Hands: 100},
		},
		BuyIn:           10,
		Fee:             1,
		AddOnAfterLevel: 1,
		AddOnBuyIn:      20,
		AddOnFee:        2,
		AddOnChips:      50,
		Payouts: []Payout{
			{Place: 1, Amount: 10},
		},
	}

	tour := mustNewTournament(t, rules)
	players := registerPlayers(t, tour, "alice", "bob", "carol", "dave")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := tour.AddOn(players[0].ID); err == nil {
		t.Fatal("AddOn succeeded before the add-on level completed")
	}

	// Bust a player during the first level, then fold out the rest of the
	// level so the add-on window opens with an eliminated entry present.
	var eliminated Standing

	for tour.Level() == 0 && eliminated.PlayerID == "" {
		before := make(map[string]bool)
		for _, standing := range tour.Standings() {
			if standing.Eliminated {
				before[standing.PlayerID] = true
			}
		}

		playIsolationHand(t, tour)

		for _, standing := range tour.Standings() {
			if standing.Eliminated && !before[standing.PlayerID] {
				eliminated = standing
			}
		}
	}

	if eliminated.PlayerID == "" {
		t.Fatal("no player busted before the first level completed")
	}

	for tour.Level() == 0 {
		playFoldHand(t, tour)
	}

	if tour.Level() != 1 || tour.HandsInLevel() != 0 {
		t.Fatalf("after level completion level=%d handsInLevel=%d, want 1/0", tour.Level(), tour.HandsInLevel())
	}

	if tour.Table().Phase() != engine.PhaseComplete {
		t.Fatalf("table phase = %s, want %s", tour.Table().Phase(), engine.PhaseComplete)
	}

	var live Standing

	for _, standing := range tour.Standings() {
		if !standing.Eliminated {
			live = standing

			break
		}
	}

	if live.PlayerID == "" {
		t.Fatalf("need a live entry, standings = %+v", tour.Standings())
	}

	if err := tour.AddOn(eliminated.PlayerID); err == nil {
		t.Fatal("AddOn succeeded for an eliminated player")
	}

	before := live.Chips

	if err := tour.AddOn(live.PlayerID); err != nil {
		t.Fatalf("AddOn(%s): %v", live.PlayerID, err)
	}

	after := standingsByID(tour)[live.PlayerID]

	if after.Chips != before+rules.AddOnChips {
		t.Fatalf("chips after add-on = %d, want %d", after.Chips, before+rules.AddOnChips)
	}

	if tour.addOns != 1 {
		t.Fatalf("add-ons = %d, want 1", tour.addOns)
	}

	if got := tour.PrizePool(); got != 4*rules.BuyIn+rules.AddOnBuyIn {
		t.Fatalf("PrizePool() = %d, want %d", got, 4*rules.BuyIn+rules.AddOnBuyIn)
	}

	if got := tour.FeesCollected(); got != 4*rules.Fee+rules.AddOnFee {
		t.Fatalf("FeesCollected() = %d, want %d", got, 4*rules.Fee+rules.AddOnFee)
	}

	if got := tour.AddOnCost(); got != 22 {
		t.Fatalf("AddOnCost() = %d, want 22", got)
	}

	if err := tour.AddOn(live.PlayerID); err == nil {
		t.Fatal("AddOn accepted a second add-on for the same entry")
	}

	if err := tour.StartHand(); err != nil {
		t.Fatalf("StartHand after the add-on: %v", err)
	}

	for _, standing := range tour.Standings() {
		if standing.Eliminated || standing.PlayerID == live.PlayerID {
			continue
		}

		if err := tour.AddOn(standing.PlayerID); err == nil {
			t.Fatal("AddOn succeeded after the next hand started")
		}

		break
	}
}

func TestAddOnBeforeAnyLevelCompletion(t *testing.T) {
	rules := lifecycleRules()
	rules.Levels[0].Hands = 1
	rules.AddOnAfterLevel = 1
	rules.AddOnBuyIn = 20
	rules.AddOnChips = 50

	tour := mustNewTournament(t, rules)
	registerPlayers(t, tour, "alice", "bob")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := tour.AddOn(tour.entries[0].player.ID); err == nil {
		t.Fatal("AddOn succeeded while the table was waiting for the first hand")
	}
}

func TestBountyAccounting(t *testing.T) {
	rules := &TournamentRules{
		StartingStack: 8,
		MinPlayers:    2,
		MaxPlayers:    2,
		Levels: []BlindLevel{
			{SmallBlind: 1, BigBlind: 2, Hands: 100},
		},
		BuyIn:  10,
		Fee:    1,
		Bounty: 2,
		Payouts: []Payout{
			{Place: 1, Amount: 20},
		},
	}

	tour := mustNewTournament(t, rules)

	if got := tour.EntryCost(); got != 13 {
		t.Fatalf("EntryCost() = %d, want 13", got)
	}

	registerPlayers(t, tour, "alice", "bob")

	if got := tour.BountyPool(); got != 4 {
		t.Fatalf("BountyPool() = %d, want 4", got)
	}

	if got := tour.BountiesPaid(); got != 0 {
		t.Fatalf("BountiesPaid() = %d before play, want 0", got)
	}

	if got := tour.PrizePool(); got != 20 {
		t.Fatalf("PrizePool() = %d, want 20 without bounties", got)
	}

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	for tour.State() == StateRunning {
		playIsolationHand(t, tour)
	}

	if tour.State() != StateFinished {
		t.Fatalf("State = %s, want %s", tour.State(), StateFinished)
	}

	standings := tour.Standings()
	if len(standings) != 2 {
		t.Fatalf("standings rows = %d, want 2", len(standings))
	}

	winner := standings[0]
	if winner.Place != 1 || winner.Eliminated {
		t.Fatalf("winner standing = %+v, want place 1 and active", winner)
	}

	if winner.BountyWon != 4 {
		t.Fatalf("winner bounty = %d, want 4 (opponent plus own bounty)", winner.BountyWon)
	}

	if got := tour.BountiesPaid(); got != 4 {
		t.Fatalf("BountiesPaid() = %d, want 4", got)
	}

	if got := tour.PrizePool(); got != 20 {
		t.Fatalf("PrizePool() = %d after the finish, want 20 without bounties", got)
	}

	if winner.Prize != 20 {
		t.Fatalf("winner prize = %d, want 20", winner.Prize)
	}

	for _, standing := range standings[1:] {
		if standing.BountyWon != 0 {
			t.Fatalf("eliminated standing = %+v, want no bounty awarded", standing)
		}
	}
}

func TestBountyLayerSelection(t *testing.T) {
	results := []engine.PotResult{
		{Amount: 100, Eligible: []string{"a"}, Winners: []string{"a"}, Refunded: true},
		{Amount: 30, Eligible: []string{"a", "b", "c"}, Winners: []string{"a"}},
		{Amount: 10, Eligible: []string{"b", "c"}, Winners: []string{"b"}},
	}

	layer, ok := bountyLayer(results, "c")
	if !ok || layer.Amount != 30 {
		t.Fatalf("bountyLayer(c) = %+v/%v, want the 30-chip main pot", layer, ok)
	}

	layer, ok = bountyLayer(results, "b")
	if !ok || layer.Amount != 30 {
		t.Fatalf("bountyLayer(b) = %+v/%v, want the largest 30-chip layer", layer, ok)
	}

	// A player only eligible for a smaller layer gets that layer.
	layer, ok = bountyLayer([]engine.PotResult{
		{Amount: 30, Eligible: []string{"a"}, Winners: []string{"a"}},
		{Amount: 10, Eligible: []string{"b"}, Winners: []string{"b"}},
	}, "b")
	if !ok || layer.Amount != 10 {
		t.Fatalf("bountyLayer(b) = %+v/%v, want the 10-chip side pot", layer, ok)
	}

	if _, ok := bountyLayer(results, "missing"); ok {
		t.Fatal("bountyLayer found a layer for an unknown player")
	}

	if _, ok := bountyLayer([]engine.PotResult{
		{Amount: 5, Eligible: []string{"a"}, Refunded: true},
	}, "a"); ok {
		t.Fatal("bountyLayer accepted a refunded layer")
	}
}

func TestSplitBountyRemainder(t *testing.T) {
	shares := splitBounty(10, []string{"a", "b", "c"}, map[string]int{"c": 0, "a": 1, "b": 2})

	if shares["c"] != 4 || shares["a"] != 3 || shares["b"] != 3 {
		t.Fatalf("shares = %+v, want c:4 a:3 b:3", shares)
	}

	// Winners missing from the seat map fall back to player ID order.
	shares = splitBounty(1, []string{"z", "a", "m"}, map[string]int{"a": 0})

	if shares["a"] != 1 || shares["m"] != 0 || shares["z"] != 0 {
		t.Fatalf("fallback shares = %+v, want the unit on the seated winner a", shares)
	}

	if got := splitBounty(5, nil, nil); len(got) != 0 {
		t.Fatalf("shares without winners = %+v, want empty", got)
	}
}

func TestAwardSurvivorBountyIncludesUnclaimed(t *testing.T) {
	winner := testEntry("winner", 0, 1)

	tour := &Tournament{
		rules:           TournamentRules{Bounty: 2},
		entries:         []*entry{winner},
		bountiesPaid:    4,
		unclaimedBounty: 6,
	}

	tour.awardSurvivorBountyLocked(winner)

	if winner.bountyWon != 8 {
		t.Fatalf("winner bounty = %d, want 8", winner.bountyWon)
	}

	if tour.bountiesPaid != 12 {
		t.Fatalf("bounties paid = %d, want 12", tour.bountiesPaid)
	}

	if tour.unclaimedBounty != 0 {
		t.Fatalf("unclaimed bounty = %d, want 0", tour.unclaimedBounty)
	}
}

func TestSitOutAndSitIn(t *testing.T) {
	rules := validRules()
	rules.StartingStack = 20
	rules.MinPlayers = 2
	rules.MaxPlayers = 3
	rules.Levels = []BlindLevel{
		{SmallBlind: 1, BigBlind: 2, Hands: 100},
	}

	tour := mustNewTournament(t, rules)
	players := registerPlayers(t, tour, "alice", "bob", "carol")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := tour.SitOut(players[0].ID); err != nil {
		t.Fatalf("SitOut before the first hand: %v", err)
	}

	standing := standingsByID(tour)[players[0].ID]
	if !standing.SittingOut {
		t.Fatalf("standing = %+v, want SittingOut", standing)
	}

	if !tour.Table().Players[0].IsSittingOut() {
		t.Fatal("table player is not marked as sitting out")
	}

	if err := tour.StartHand(); err != nil {
		t.Fatalf("StartHand with a sitting-out player: %v", err)
	}

	if cards := tour.Table().Players[0].GetPocketCards(); len(cards) != 0 {
		t.Fatalf("sitting-out player received %d pocket cards, want 0", len(cards))
	}

	foldToShowdown(t, tour)

	if _, err := tour.Showdown(); err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	standing = standingsByID(tour)[players[0].ID]
	if standing.Chips != 20 {
		t.Fatalf("sitting-out player chips = %d, want 20 (no blinds or antes)", standing.Chips)
	}

	if err := tour.SitIn(players[0].ID); err != nil {
		t.Fatalf("SitIn: %v", err)
	}

	if standingsByID(tour)[players[0].ID].SittingOut {
		t.Fatal("standing still reports SittingOut after SitIn")
	}

	if err := tour.StartHand(); err != nil {
		t.Fatalf("StartHand after SitIn: %v", err)
	}

	if cards := tour.Table().Players[0].GetPocketCards(); len(cards) != 2 {
		t.Fatalf("sitting-in player received %d pocket cards, want 2", len(cards))
	}
}

func TestSitOutErrors(t *testing.T) {
	rules := validRules()
	rules.StartingStack = 20
	rules.Levels = []BlindLevel{
		{SmallBlind: 1, BigBlind: 2, Hands: 100},
	}

	tour := mustNewTournament(t, rules)
	players := registerPlayers(t, tour, "alice", "bob", "carol")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := tour.StartHand(); err != nil {
		t.Fatalf("StartHand: %v", err)
	}

	if err := tour.SitOut(players[0].ID); err == nil {
		t.Fatal("SitOut succeeded during a hand")
	}

	if err := tour.SitIn(players[0].ID); err == nil {
		t.Fatal("SitIn succeeded during a hand")
	}

	foldToShowdown(t, tour)

	if _, err := tour.Showdown(); err != nil {
		t.Fatalf("Showdown: %v", err)
	}

	if err := tour.SitOut("missing"); err == nil {
		t.Fatal("SitOut accepted an unknown player ID")
	}

	if err := tour.SitIn("missing"); err == nil {
		t.Fatal("SitIn accepted an unknown player ID")
	}
}

func TestSitOutLeavesTooFewActivePlayers(t *testing.T) {
	rules := validRules()
	rules.StartingStack = 20
	rules.MinPlayers = 2
	rules.MaxPlayers = 2
	rules.Levels = []BlindLevel{
		{SmallBlind: 1, BigBlind: 2, Hands: 100},
	}

	tour := mustNewTournament(t, rules)
	players := registerPlayers(t, tour, "alice", "bob")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := tour.SitOut(players[0].ID); err != nil {
		t.Fatalf("SitOut: %v", err)
	}

	if err := tour.StartHand(); err == nil {
		t.Fatal("StartHand succeeded with only one active player")
	}

	if err := tour.SitIn(players[0].ID); err != nil {
		t.Fatalf("SitIn: %v", err)
	}

	if err := tour.StartHand(); err != nil {
		t.Fatalf("StartHand after SitIn: %v", err)
	}
}

func TestSitOutEliminatedPlayerRejected(t *testing.T) {
	tour := mustNewTournament(t, lifecycleRules())
	registerPlayers(t, tour, "alice", "bob", "carol", "dave")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	busted := bustOneRunning(t, tour)

	if err := tour.SitOut(busted.PlayerID); err == nil {
		t.Fatal("SitOut succeeded for an eliminated player")
	}

	if err := tour.SitIn(busted.PlayerID); err == nil {
		t.Fatal("SitIn succeeded for an eliminated player")
	}
}

func TestLifecycleRulesValidation(t *testing.T) {
	tests := []struct {
		name  string
		rules func() *TournamentRules
	}{
		{
			name: "negative late registration level",
			rules: func() *TournamentRules {
				r := validRules()
				r.LateRegUntilLevel = -1
				return r
			},
		},
		{
			name: "late registration beyond the level count",
			rules: func() *TournamentRules {
				r := validRules()
				r.LateRegUntilLevel = 2
				return r
			},
		},
		{
			name: "negative max re-entries",
			rules: func() *TournamentRules {
				r := validRules()
				r.MaxReentries = -1
				return r
			},
		},
		{
			name: "max re-entries without re-entry",
			rules: func() *TournamentRules {
				r := validRules()
				r.MaxReentries = 1
				return r
			},
		},
		{
			name: "re-entry without late registration",
			rules: func() *TournamentRules {
				r := validRules()
				r.ReentryAllowed = true
				return r
			},
		},
		{
			name: "negative add-on level",
			rules: func() *TournamentRules {
				r := validRules()
				r.AddOnAfterLevel = -1
				return r
			},
		},
		{
			name: "add-on level has no next level",
			rules: func() *TournamentRules {
				r := validRules()
				r.AddOnAfterLevel = 1
				r.AddOnBuyIn = 10
				r.AddOnChips = 10
				return r
			},
		},
		{
			name: "add-on without a buy-in",
			rules: func() *TournamentRules {
				r := validRules()
				r.AddOnAfterLevel = 1
				r.AddOnChips = 10
				r.Levels = append(r.Levels, BlindLevel{SmallBlind: 2, BigBlind: 4, Hands: 2})
				return r
			},
		},
		{
			name: "negative add-on fee",
			rules: func() *TournamentRules {
				r := validRules()
				r.AddOnAfterLevel = 1
				r.AddOnBuyIn = 10
				r.AddOnFee = -1
				r.AddOnChips = 10
				r.Levels = append(r.Levels, BlindLevel{SmallBlind: 2, BigBlind: 4, Hands: 2})
				return r
			},
		},
		{
			name: "add-on without chips",
			rules: func() *TournamentRules {
				r := validRules()
				r.AddOnAfterLevel = 1
				r.AddOnBuyIn = 10
				r.Levels = append(r.Levels, BlindLevel{SmallBlind: 2, BigBlind: 4, Hands: 2})
				return r
			},
		},
		{
			name: "add-on pricing without a level",
			rules: func() *TournamentRules {
				r := validRules()
				r.AddOnChips = 10
				return r
			},
		},
		{
			name: "negative bounty",
			rules: func() *TournamentRules {
				r := validRules()
				r.Bounty = -1
				return r
			},
		},
		{
			name: "bounty does not fund the prize pool",
			rules: func() *TournamentRules {
				r := percentRules()
				r.BuyIn = 0
				r.Bounty = 5
				return r
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewTournament("bad", "tag", nil, tt.rules()); err == nil {
				t.Fatal("NewTournament accepted invalid lifecycle rules")
			}
		})
	}
}

func TestAddOnBuyInEnablesAccounting(t *testing.T) {
	rules := validRules()
	rules.Payouts = nil
	rules.PayoutMode = PayoutModePercent
	rules.PercentPayouts = []PercentPayout{{Place: 1, Percent: 100}}
	rules.Levels = append(rules.Levels, BlindLevel{SmallBlind: 2, BigBlind: 4, Hands: 2})
	rules.AddOnAfterLevel = 1
	rules.AddOnBuyIn = 10
	rules.AddOnChips = 100

	normalized, err := normalizeRules(rules)
	if err != nil {
		t.Fatalf("normalizeRules with an add-on-funded percent payout: %v", err)
	}

	if !normalized.accountingEnabled() {
		t.Fatal("accountingEnabled() ignored AddOnBuyIn")
	}
}

func TestLifecycleRulesArePreserved(t *testing.T) {
	rules := validRules()
	rules.Levels = append(rules.Levels, BlindLevel{SmallBlind: 2, BigBlind: 4, Hands: 2})
	rules.LateRegUntilLevel = 2
	rules.ReentryAllowed = true
	rules.MaxReentries = 3
	rules.AddOnAfterLevel = 1
	rules.AddOnBuyIn = 20
	rules.AddOnFee = 2
	rules.AddOnChips = 500
	rules.Bounty = 5

	tour := mustNewTournament(t, rules)
	got := tour.Rules()

	if got.LateRegUntilLevel != 2 ||
		!got.ReentryAllowed ||
		got.MaxReentries != 3 ||
		got.AddOnAfterLevel != 1 ||
		got.AddOnBuyIn != 20 ||
		got.AddOnFee != 2 ||
		got.AddOnChips != 500 ||
		got.Bounty != 5 {
		t.Fatalf("Rules() = %+v, want the lifecycle fields preserved", got)
	}

	if got := tour.EntryCost(); got != 5 {
		t.Fatalf("EntryCost() = %d, want BuyIn+Fee+Bounty = 5", got)
	}

	if got := tour.AddOnCost(); got != 22 {
		t.Fatalf("AddOnCost() = %d, want 22", got)
	}
}

func TestZeroValueLifecycleRulesRemainCompatible(t *testing.T) {
	tour := mustNewTournament(t, validRules())
	got := tour.Rules()

	if got.LateRegUntilLevel != 0 ||
		got.ReentryAllowed ||
		got.MaxReentries != 0 ||
		got.AddOnAfterLevel != 0 ||
		got.AddOnBuyIn != 0 ||
		got.AddOnFee != 0 ||
		got.AddOnChips != 0 ||
		got.Bounty != 0 {
		t.Fatalf("zero-value rules gained lifecycle defaults: %+v", got)
	}

	registerPlayers(t, tour, "alice", "bob")

	if err := tour.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, err := tour.Register("late"); err == nil {
		t.Fatal("zero-value rules accepted late registration")
	}

	if got := tour.EntryCost(); got != 0 {
		t.Fatalf("EntryCost() = %d, want 0", got)
	}

	if got := tour.BountyPool(); got != 0 {
		t.Fatalf("BountyPool() = %d, want 0", got)
	}
}

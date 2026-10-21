package models

import (
	"sync"
	"testing"
)

// stubIDMaker returns a fixed ID so tests do not depend on randomness.
type stubIDMaker struct {
	id string
}

func (s stubIDMaker) MakeID() string {
	return s.id
}

func TestNewPlayer(t *testing.T) {
	t.Run("nil id maker falls back to default", func(t *testing.T) {
		p := NewPlayer("alice", nil, 100)

		if p.ID == "" {
			t.Fatal("NewPlayer with nil idMaker produced an empty ID")
		}

		if p.Name != "alice" {
			t.Fatalf("Name = %q, want %q", p.Name, "alice")
		}

		if !p.Active {
			t.Fatal("new player is not active")
		}

		if got := p.GetCurrentChipsAmount(); got != 100 {
			t.Fatalf("GetCurrentChipsAmount() = %d, want 100", got)
		}
	})

	t.Run("negative chips clamped to zero", func(t *testing.T) {
		p := NewPlayer("bob", stubIDMaker{"bob"}, -50)

		if got := p.GetCurrentChipsAmount(); got != 0 {
			t.Fatalf("GetCurrentChipsAmount() = %d, want 0", got)
		}
	})
}

func TestAddCardPocketLimit(t *testing.T) {
	p := NewPlayer("alice", stubIDMaker{"alice"}, 100)
	cards := mustCards(t, "2C", "3C", "4C")

	for i := 0; i < PocketSize; i++ {
		if err := p.AddCard(cards[i]); err != nil {
			t.Fatalf("AddCard(%d): %v", i, err)
		}
	}

	if err := p.AddCard(cards[2]); err == nil {
		t.Fatal("AddCard accepted a third pocket card")
	}

	if got := len(p.GetPocketCards()); got != PocketSize {
		t.Fatalf("len(GetPocketCards()) = %d, want %d", got, PocketSize)
	}
}

func TestAddCardNilCard(t *testing.T) {
	p := NewPlayer("alice", stubIDMaker{"alice"}, 100)

	if err := p.AddCard(nil); err == nil {
		t.Fatal("AddCard(nil) succeeded")
	}

	if got := len(p.GetPocketCards()); got != 0 {
		t.Fatalf("len(GetPocketCards()) = %d, want 0", got)
	}
}

func TestGetPocketCardsReturnsCopy(t *testing.T) {
	p := NewPlayer("alice", stubIDMaker{"alice"}, 100)
	cards := mustCards(t, "2C", "3C")

	if err := p.AddCard(cards[0]); err != nil {
		t.Fatalf("AddCard: %v", err)
	}

	if err := p.AddCard(cards[1]); err != nil {
		t.Fatalf("AddCard: %v", err)
	}

	got := p.GetPocketCards()
	if len(got) != PocketSize {
		t.Fatalf("len(GetPocketCards()) = %d, want %d", len(got), PocketSize)
	}

	// Mutating the returned slice must not affect the player.
	got[0] = nil
	got[1] = nil

	fresh := p.GetPocketCards()
	for i, c := range fresh {
		if c == nil {
			t.Fatalf("fresh[%d] is nil: returned slice aliases internal state", i)
		}
	}
}

func TestCall(t *testing.T) {
	tests := []struct {
		name      string
		stack     Chips
		bet       *Bet
		wantAmt   Chips
		wantErr   bool
		wantStack Chips
	}{
		{"deducts bet", 100, NewBet(30, 0), 30, false, 70},
		{"exact stack allowed", 50, NewBet(50, 0), 50, false, 0},
		{"over stack rejected", 10, NewBet(11, 0), 0, true, 10},
		{"negative bet rejected", 10, NewBet(-1, 0), 0, true, 10},
		{"nil bet rejected", 10, nil, 0, true, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPlayer("alice", stubIDMaker{"alice"}, tt.stack)

			amt, err := p.Call(tt.bet)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Call() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && amt != tt.wantAmt {
				t.Fatalf("Call() = %d, want %d", amt, tt.wantAmt)
			}

			if got := p.GetCurrentChipsAmount(); got != tt.wantStack {
				t.Fatalf("GetCurrentChipsAmount() = %d, want %d", got, tt.wantStack)
			}
		})
	}
}

func TestRaise(t *testing.T) {
	tests := []struct {
		name      string
		stack     Chips
		bet       *Bet
		wantAmt   Chips
		wantErr   bool
		wantStack Chips
	}{
		{"deducts bet and over", 100, NewBet(20, 30), 50, false, 50},
		{"total equals stack", 50, NewBet(20, 30), 50, false, 0},
		{"total exceeds stack rejected", 49, NewBet(20, 30), 0, true, 49},
		{"negative bet rejected", 100, NewBet(-1, 10), 0, true, 100},
		{"negative over rejected", 100, NewBet(10, -5), 0, true, 100},
		{"nil bet rejected", 100, nil, 0, true, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPlayer("alice", stubIDMaker{"alice"}, tt.stack)

			amt, err := p.Raise(tt.bet)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Raise() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && amt != tt.wantAmt {
				t.Fatalf("Raise() = %d, want %d", amt, tt.wantAmt)
			}

			if got := p.GetCurrentChipsAmount(); got != tt.wantStack {
				t.Fatalf("GetCurrentChipsAmount() = %d, want %d", got, tt.wantStack)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name      string
		stack     Chips
		bet       *Bet
		wantAmt   Chips
		wantErr   bool
		wantStack Chips
	}{
		{"returns amount without deducting", 100, NewBet(40, 0), 40, false, 100},
		{"nil bet rejected", 100, nil, 0, true, 100},
		{"negative bet rejected", 100, NewBet(-5, 0), 0, true, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPlayer("alice", stubIDMaker{"alice"}, tt.stack)

			amt, err := p.Check(tt.bet)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && amt != tt.wantAmt {
				t.Fatalf("Check() = %d, want %d", amt, tt.wantAmt)
			}

			if got := p.GetCurrentChipsAmount(); got != tt.wantStack {
				t.Fatalf("GetCurrentChipsAmount() = %d, want %d", got, tt.wantStack)
			}
		})
	}
}

func TestFall(t *testing.T) {
	t.Run("nil bet rejected", func(t *testing.T) {
		p := NewPlayer("alice", stubIDMaker{"alice"}, 100)

		if _, err := p.Fall(nil); err == nil {
			t.Fatal("Fall(nil) succeeded")
		}

		if !p.Active {
			t.Fatal("player deactivated by rejected Fall")
		}
	})

	t.Run("deactivates and clears cards", func(t *testing.T) {
		p := NewPlayer("alice", stubIDMaker{"alice"}, 100)
		cards := mustCards(t, "2C", "3C")

		for _, c := range cards {
			if err := p.AddCard(c); err != nil {
				t.Fatalf("AddCard: %v", err)
			}
		}

		amt, err := p.Fall(NewBet(25, 0))
		if err != nil {
			t.Fatalf("Fall: %v", err)
		}

		if amt != 25 {
			t.Fatalf("Fall() = %d, want 25", amt)
		}

		if p.Active {
			t.Fatal("player is still active after Fall")
		}

		if got := len(p.GetPocketCards()); got != 0 {
			t.Fatalf("len(GetPocketCards()) = %d, want 0", got)
		}

		if got := p.GetCurrentChipsAmount(); got != 100 {
			t.Fatalf("GetCurrentChipsAmount() = %d, want 100 (chips not deducted)", got)
		}
	})
}

func TestChipsNeverNegative(t *testing.T) {
	p := NewPlayer("alice", stubIDMaker{"alice"}, 10)

	if _, err := p.Call(NewBet(11, 0)); err == nil {
		t.Fatal("over-stack Call succeeded")
	}

	if _, err := p.Raise(NewBet(5, 6)); err == nil {
		t.Fatal("over-stack Raise succeeded")
	}

	if got := p.GetCurrentChipsAmount(); got < 0 {
		t.Fatalf("GetCurrentChipsAmount() = %d, want >= 0", got)
	}

	if got := p.GetCurrentChipsAmount(); got != 10 {
		t.Fatalf("GetCurrentChipsAmount() = %d, want 10", got)
	}
}

func TestPlayerConcurrentAddCard(t *testing.T) {
	p := NewPlayer("alice", stubIDMaker{"alice"}, 100)
	cards := mustCards(t, "2C", "3C", "4C", "5C", "6C", "7C", "8C", "9C", "10C", "JC")

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0

	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			if err := p.AddCard(cards[i]); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if successes != PocketSize {
		t.Fatalf("successful AddCard calls = %d, want %d", successes, PocketSize)
	}

	if got := len(p.GetPocketCards()); got != PocketSize {
		t.Fatalf("len(GetPocketCards()) = %d, want %d", got, PocketSize)
	}
}

func TestPlayerConcurrentCall(t *testing.T) {
	p := NewPlayer("alice", stubIDMaker{"alice"}, 5)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0

	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if amt, err := p.Call(NewBet(1, 0)); err == nil {
				if amt != 1 {
					t.Errorf("Call() = %d, want 1", amt)
				}

				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if successes != 5 {
		t.Fatalf("successful Call() count = %d, want 5", successes)
	}

	if got := p.GetCurrentChipsAmount(); got != 0 {
		t.Fatalf("GetCurrentChipsAmount() = %d, want 0", got)
	}
}

func TestPlayerCommitAndAllIn(t *testing.T) {
	p := NewPlayer("alice", stubIDMaker{"alice"}, 100)

	paid, err := p.commit(40)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	if paid != 40 {
		t.Fatalf("commit paid = %d, want 40", paid)
	}

	if got := p.GetCurrentChipsAmount(); got != 60 {
		t.Fatalf("stack = %d, want 60", got)
	}

	if got := p.GetStreetBet(); got != 40 {
		t.Fatalf("street bet = %d, want 40", got)
	}

	if p.IsAllIn() {
		t.Fatal("player is all-in too early")
	}

	if _, err := p.commit(61); err == nil {
		t.Fatal("commit above stack succeeded")
	}

	if p.GetCurrentChipsAmount() != 60 || p.GetStreetBet() != 40 {
		t.Fatal("rejected commit changed player state")
	}

	if _, err := p.commit(60); err != nil {
		t.Fatalf("all-in commit: %v", err)
	}

	if !p.IsAllIn() {
		t.Fatal("player is not all-in at zero chips")
	}

	p.resetForStreet()

	if got := p.GetStreetBet(); got != 0 {
		t.Fatalf("street bet after reset = %d, want 0", got)
	}

	if !p.IsAllIn() {
		t.Fatal("resetForStreet cleared the all-in flag")
	}
}

func TestPlayerResetForHandSitsOutEmptyStack(t *testing.T) {
	broke := NewPlayer("broke", stubIDMaker{"broke"}, 0)
	broke.Active = true

	broke.ResetForHand()

	if broke.Active {
		t.Fatal("empty-stack player is active after ResetForHand")
	}

	funded := NewPlayer("funded", stubIDMaker{"funded"}, 10)
	funded.Active = false

	if err := funded.AddCard(mustCards(t, "AS")[0]); err != nil {
		t.Fatalf("AddCard: %v", err)
	}

	if _, err := funded.commit(5); err != nil {
		t.Fatalf("commit: %v", err)
	}

	funded.ResetForHand()

	if !funded.Active {
		t.Fatal("funded player is inactive after ResetForHand")
	}

	if got := len(funded.GetPocketCards()); got != 0 {
		t.Fatalf("pocket cards after ResetForHand = %d, want 0", got)
	}

	if funded.GetStreetBet() != 0 || funded.HasActed() || funded.IsAllIn() {
		t.Fatal("ResetForHand did not clear per-hand state")
	}
}

func TestPlayerFoldKeepsCards(t *testing.T) {
	p := NewPlayer("alice", stubIDMaker{"alice"}, 100)

	if err := p.AddCard(mustCards(t, "AS")[0]); err != nil {
		t.Fatalf("AddCard: %v", err)
	}

	p.fold()

	if p.Active {
		t.Fatal("player is active after fold")
	}

	if !p.HasActed() {
		t.Fatal("folded player has not acted")
	}

	if got := len(p.GetPocketCards()); got != 1 {
		t.Fatalf("fold cleared pocket cards: got %d, want 1", got)
	}
}

func TestPlayerConcurrentAddCardAndCall(t *testing.T) {
	p := NewPlayer("alice", stubIDMaker{"alice"}, 10)
	cards := mustCards(t, "2C", "3C", "4C", "5C", "6C")

	var wg sync.WaitGroup
	var addMu sync.Mutex
	var callMu sync.Mutex
	addSuccesses := 0
	callSuccesses := 0

	for i := 0; i < 5; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			if err := p.AddCard(cards[i]); err == nil {
				addMu.Lock()
				addSuccesses++
				addMu.Unlock()
			}

			if _, err := p.Call(NewBet(1, 0)); err == nil {
				callMu.Lock()
				callSuccesses++
				callMu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if addSuccesses != PocketSize {
		t.Fatalf("successful AddCard calls = %d, want %d", addSuccesses, PocketSize)
	}

	if callSuccesses != 5 {
		t.Fatalf("successful Call() count = %d, want 5", callSuccesses)
	}

	if got := len(p.GetPocketCards()); got != PocketSize {
		t.Fatalf("len(GetPocketCards()) = %d, want %d", got, PocketSize)
	}

	if got := p.GetCurrentChipsAmount(); got != 5 {
		t.Fatalf("GetCurrentChipsAmount() = %d, want 5", got)
	}
}

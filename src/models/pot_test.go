package models

import "testing"

func TestPotRegister(t *testing.T) {
	p := NewPot("main")

	if err := p.Register("p1"); err != nil {
		t.Fatalf("Register(p1): %v", err)
	}

	if got := p.PlayersChips["p1"]; got != 0 {
		t.Fatalf("PlayersChips[p1] = %d, want 0", got)
	}

	if err := p.Register("p1"); err == nil {
		t.Fatal("duplicate Register(p1) succeeded")
	}

	if got := p.PlayersChips["p1"]; got != 0 {
		t.Fatalf("PlayersChips[p1] = %d after rejected duplicate, want 0", got)
	}
}

func TestPotAddPlayerBet(t *testing.T) {
	p := NewPot("main")

	if err := p.Register("p1"); err != nil {
		t.Fatalf("Register(p1): %v", err)
	}

	if err := p.Register("p2"); err != nil {
		t.Fatalf("Register(p2): %v", err)
	}

	tests := []struct {
		name   string
		bet    Chips
		player string
		want   int
	}{
		{"first bet", 10, "p1", 10},
		{"second bet accumulates", 15, "p1", 25},
		{"other player independent", 5, "p2", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.AddPlayerBet(tt.bet, tt.player)
			if err != nil {
				t.Fatalf("AddPlayerBet(%d, %s): %v", tt.bet, tt.player, err)
			}

			if got != tt.want {
				t.Fatalf("AddPlayerBet() = %d, want %d", got, tt.want)
			}
		})
	}

	if p.TotalChipsNum != 30 {
		t.Fatalf("TotalChipsNum = %d, want 30", p.TotalChipsNum)
	}

	if got := p.PlayersChips["p1"]; got != 25 {
		t.Fatalf("PlayersChips[p1] = %d, want 25", got)
	}

	if got := p.PlayersChips["p2"]; got != 5 {
		t.Fatalf("PlayersChips[p2] = %d, want 5", got)
	}
}

func TestPotAddPlayerBetErrors(t *testing.T) {
	p := NewPot("main")

	if err := p.Register("p1"); err != nil {
		t.Fatalf("Register(p1): %v", err)
	}

	if _, err := p.AddPlayerBet(10, "p1"); err != nil {
		t.Fatalf("AddPlayerBet(10, p1): %v", err)
	}

	t.Run("unknown player errors", func(t *testing.T) {
		if _, err := p.AddPlayerBet(5, "nope"); err == nil {
			t.Fatal("AddPlayerBet for unknown player succeeded")
		}

		if p.TotalChipsNum != 10 {
			t.Fatalf("TotalChipsNum = %d, want 10", p.TotalChipsNum)
		}
	})

	t.Run("negative bet rejected", func(t *testing.T) {
		if _, err := p.AddPlayerBet(-5, "p1"); err == nil {
			t.Fatal("negative AddPlayerBet succeeded")
		}

		if got := p.PlayersChips["p1"]; got != 10 {
			t.Fatalf("PlayersChips[p1] = %d, want 10", got)
		}

		if p.TotalChipsNum != 10 {
			t.Fatalf("TotalChipsNum = %d, want 10", p.TotalChipsNum)
		}
	})
}

func TestPotGetPlayerAmount(t *testing.T) {
	p := NewPot("main")

	if err := p.Register("p1"); err != nil {
		t.Fatalf("Register(p1): %v", err)
	}

	if got, err := p.GetPlayerAmount("p1"); err != nil || got != 0 {
		t.Fatalf("GetPlayerAmount(p1) = %d, %v; want 0, nil", got, err)
	}

	if _, err := p.AddPlayerBet(42, "p1"); err != nil {
		t.Fatalf("AddPlayerBet: %v", err)
	}

	if got, err := p.GetPlayerAmount("p1"); err != nil || got != 42 {
		t.Fatalf("GetPlayerAmount(p1) = %d, %v; want 42, nil", got, err)
	}

	if _, err := p.GetPlayerAmount("nope"); err == nil {
		t.Fatal("GetPlayerAmount for unknown player succeeded")
	}
}

func TestPotReset(t *testing.T) {
	p := NewPot("main")

	if err := p.Register("p1"); err != nil {
		t.Fatalf("Register(p1): %v", err)
	}

	if err := p.Register("p2"); err != nil {
		t.Fatalf("Register(p2): %v", err)
	}

	if _, err := p.AddPlayerBet(10, "p1"); err != nil {
		t.Fatalf("AddPlayerBet: %v", err)
	}

	if _, err := p.AddPlayerBet(20, "p2"); err != nil {
		t.Fatalf("AddPlayerBet: %v", err)
	}

	p.Reset()

	if len(p.PlayersChips) != 0 {
		t.Fatalf("len(PlayersChips) = %d, want 0", len(p.PlayersChips))
	}

	if p.TotalChipsNum != 0 {
		t.Fatalf("TotalChipsNum = %d, want 0", p.TotalChipsNum)
	}
}

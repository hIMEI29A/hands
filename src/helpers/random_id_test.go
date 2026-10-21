package helpers

import "testing"

// isLowercaseHex reports whether s consists only of lowercase hex digits.
func isLowercaseHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}

	return true
}

func TestGenerateRandomIDLength(t *testing.T) {
	for _, length := range []int{0, 1, 2, 7, 24, 25} {
		id, err := generateRandomID(length)
		if err != nil {
			t.Fatalf("generateRandomID(%d) returned error: %v", length, err)
		}

		if len(id) != length {
			t.Errorf("generateRandomID(%d) length = %d, want %d", length, len(id), length)
		}

		if !isLowercaseHex(id) {
			t.Errorf("generateRandomID(%d) = %q, want only lowercase hex characters", length, id)
		}
	}
}

func TestGenerateRandomIDNegativeLength(t *testing.T) {
	for _, length := range []int{-1, -24} {
		id, err := generateRandomID(length)
		if err != nil {
			t.Fatalf("generateRandomID(%d) returned error: %v", length, err)
		}

		if id != "" {
			t.Errorf("generateRandomID(%d) = %q, want an empty string", length, id)
		}
	}
}

func TestGenerateRandomIDConsecutiveCallsDiffer(t *testing.T) {
	// Only lengths with enough entropy are checked pairwise: a 1-character
	// id has just 16 possible values, so consecutive ids may collide.
	for _, length := range []int{7, 24, 25} {
		prev, err := generateRandomID(length)
		if err != nil {
			t.Fatalf("generateRandomID(%d) returned error: %v", length, err)
		}

		for i := 0; i < 100; i++ {
			id, err := generateRandomID(length)
			if err != nil {
				t.Fatalf("generateRandomID(%d) returned error: %v", length, err)
			}

			if id == prev {
				t.Fatalf("generateRandomID(%d) returned the same id twice in a row: %q", length, id)
			}

			prev = id
		}
	}
}

func TestGenerateRandomIDShortLengthsVary(t *testing.T) {
	for _, length := range []int{1, 2} {
		seen := make(map[string]struct{}, 100)

		for i := 0; i < 100; i++ {
			id, err := generateRandomID(length)
			if err != nil {
				t.Fatalf("generateRandomID(%d) returned error: %v", length, err)
			}

			seen[id] = struct{}{}
		}

		if len(seen) < 2 {
			t.Errorf("generateRandomID(%d) produced only %d distinct ids in 100 calls", length, len(seen))
		}
	}
}

func TestGenerateDefaultRandomID(t *testing.T) {
	id := GenerateDefaultRandomID()

	if len(id) != defaultIdLength {
		t.Errorf("GenerateDefaultRandomID() length = %d, want %d", len(id), defaultIdLength)
	}

	if !isLowercaseHex(id) {
		t.Errorf("GenerateDefaultRandomID() = %q, want only lowercase hex characters", id)
	}

	if other := GenerateDefaultRandomID(); other == id {
		t.Errorf("GenerateDefaultRandomID() returned the same id twice: %q", id)
	}
}

func TestDefaultIdGeneratorMakeID(t *testing.T) {
	g := NewDefaultIdGenerator()

	id := g.MakeID()

	if len(id) != defaultIdLength {
		t.Errorf("MakeID() length = %d, want %d", len(id), defaultIdLength)
	}

	if !isLowercaseHex(id) {
		t.Errorf("MakeID() = %q, want only lowercase hex characters", id)
	}

	if other := g.MakeID(); other == id {
		t.Errorf("MakeID() returned the same id twice: %q", id)
	}
}

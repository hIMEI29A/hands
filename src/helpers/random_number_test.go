package helpers

import "testing"

func TestGenerateRandomNumInRangeWithinBounds(t *testing.T) {
	for _, max := range []int{1, 2, 7, 13, 52, 1000} {
		for i := 0; i < 1000; i++ {
			got := GenerateRandomNumInRange(max)

			if got < 0 || got >= max {
				t.Fatalf("GenerateRandomNumInRange(%d) = %d after %d calls, want value in [0, %d)", max, got, i, max)
			}
		}
	}
}

func TestGenerateRandomNumInRangeValuesVary(t *testing.T) {
	seen := make(map[int]struct{})

	for i := 0; i < 1000; i++ {
		seen[GenerateRandomNumInRange(4)] = struct{}{}
	}

	if len(seen) < 2 {
		t.Errorf("GenerateRandomNumInRange(4) produced only %d distinct values in 1000 calls", len(seen))
	}
}

func TestGenerateRandomNumInRangeMaxOne(t *testing.T) {
	for i := 0; i < 100; i++ {
		if got := GenerateRandomNumInRange(1); got != 0 {
			t.Fatalf("GenerateRandomNumInRange(1) = %d, want 0", got)
		}
	}
}

func TestGenerateRandomNumInRangeNonPositiveMax(t *testing.T) {
	testCases := []int{0, -1, -100}

	for _, max := range testCases {
		if got := GenerateRandomNumInRange(max); got != 0 {
			t.Errorf("GenerateRandomNumInRange(%d) = %d, want 0 without panicking", max, got)
		}
	}
}

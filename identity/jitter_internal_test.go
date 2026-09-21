package identity

import (
	"errors"
	"testing"
)

type noEntropy struct{}

func (noEntropy) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

func TestRandomUnitFallsBackWithoutEntropy(t *testing.T) {
	prev := randReader
	randReader = noEntropy{}
	defer func() { randReader = prev }()
	if got := randomUnit(); got != 0.5 {
		t.Fatalf("expected midpoint fallback, got %v", got)
	}
	randReader = prev
	for i := 0; i < 100; i++ {
		if u := randomUnit(); u < 0 || u >= 1 {
			t.Fatalf("out of range: %v", u)
		}
	}
}

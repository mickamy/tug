package port_test

import (
	"testing"

	"github.com/mickamy/tug/internal/port"
)

func TestCompute_Deterministic(t *testing.T) {
	t.Parallel()

	a := port.Compute("proj", "postgres", 5432, nil)
	b := port.Compute("proj", "postgres", 5432, nil)
	if a != b {
		t.Errorf("expected deterministic result, got %d and %d", a, b)
	}
}

func TestCompute_InRange(t *testing.T) {
	t.Parallel()

	p := port.Compute("proj", "svc", 3000, nil)
	if p < 10000 || p >= 60000 {
		t.Errorf("port %d out of range [10000, 60000)", p)
	}
}

func TestCompute_DifferentInputsDifferentPorts(t *testing.T) {
	t.Parallel()

	a := port.Compute("app-a", "postgres", 5432, nil)
	b := port.Compute("app-b", "postgres", 5432, nil)
	if a == b {
		t.Errorf("different projects should (likely) produce different ports, both got %d", a)
	}
}

func TestCompute_AvoidsUsed(t *testing.T) {
	t.Parallel()

	first := port.Compute("proj", "svc", 5432, nil)

	used := map[uint16]struct{}{first: {}}
	second := port.Compute("proj", "svc", 5432, used)

	if second == first {
		t.Errorf("expected different port when first is marked used, both got %d", first)
	}
	if second < 10000 || second >= 60000 {
		t.Errorf("port %d out of range [10000, 60000)", second)
	}
}

package port_test

import (
	"errors"
	"testing"

	"github.com/mickamy/tug/internal/port"
)

func TestCompute_Deterministic(t *testing.T) {
	t.Parallel()

	a, err := port.Compute("proj", "postgres", 5432, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := port.Compute("proj", "postgres", 5432, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != b {
		t.Errorf("expected deterministic result, got %d and %d", a, b)
	}
}

func TestCompute_InRange(t *testing.T) {
	t.Parallel()

	p, err := port.Compute("proj", "svc", 3000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p < 10000 || p >= 60000 {
		t.Errorf("port %d out of range [10000, 60000)", p)
	}
}

func TestCompute_DifferentInputsDifferentPorts(t *testing.T) {
	t.Parallel()

	a, err := port.Compute("app-a", "postgres", 5432, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := port.Compute("app-b", "postgres", 5432, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == b {
		t.Errorf("different projects should (likely) produce different ports, both got %d", a)
	}
}

func TestCompute_AvoidsUsed(t *testing.T) {
	t.Parallel()

	first, err := port.Compute("proj", "svc", 5432, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	used := map[uint16]struct{}{first: {}}
	second, err := port.Compute("proj", "svc", 5432, used)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if second == first {
		t.Errorf("expected different port when first is marked used, both got %d", first)
	}
	if second < 10000 || second >= 60000 {
		t.Errorf("port %d out of range [10000, 60000)", second)
	}
}

func TestCompute_AllUsed(t *testing.T) {
	t.Parallel()

	used := make(map[uint16]struct{})
	for p := uint16(10000); p < 60000; p++ {
		used[p] = struct{}{}
	}

	_, err := port.Compute("proj", "svc", 5432, used)
	if err == nil {
		t.Fatal("expected error when all ports are used")
	}
	if !errors.Is(err, port.ErrNoAvailablePort) {
		t.Fatalf("expected ErrNoAvailablePort, got %v", err)
	}
}

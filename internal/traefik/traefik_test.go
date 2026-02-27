package traefik_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickamy/tug/internal/traefik"
)

// mockRunner records calls and returns preconfigured responses.
type mockRunner struct {
	runtimeCalls       [][]string
	runtimeOutputCalls [][]string

	runtimeFunc       func(args []string) error
	runtimeOutputFunc func(args []string) ([]byte, error)
}

func (m *mockRunner) Runtime(_ context.Context, args ...string) error {
	m.runtimeCalls = append(m.runtimeCalls, args)
	if m.runtimeFunc != nil {
		return m.runtimeFunc(args)
	}
	return nil
}

func (m *mockRunner) RuntimeOutput(_ context.Context, args ...string) ([]byte, error) {
	m.runtimeOutputCalls = append(m.runtimeOutputCalls, args)
	if m.runtimeOutputFunc != nil {
		return m.runtimeOutputFunc(args)
	}
	return nil, nil
}

func (m *mockRunner) Compose(_ context.Context, _ ...string) error { return nil }
func (m *mockRunner) ComposeOutput(_ context.Context, _ ...string) ([]byte, error) {
	return nil, nil
}

func TestEnsureNetwork_AlreadyExists(t *testing.T) {
	t.Parallel()

	m := &mockRunner{
		runtimeOutputFunc: func(args []string) ([]byte, error) {
			// "network inspect tug" succeeds → network exists
			return []byte("{}"), nil
		},
	}

	if err := traefik.EnsureNetwork(t.Context(), m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.runtimeCalls) != 0 {
		t.Errorf("expected no Runtime calls, got %d: %v", len(m.runtimeCalls), m.runtimeCalls)
	}
}

func TestEnsureNetwork_Creates(t *testing.T) {
	t.Parallel()

	m := &mockRunner{
		runtimeOutputFunc: func(args []string) ([]byte, error) {
			// "network inspect tug" fails → network does not exist
			return nil, errors.New("No such network: tug")
		},
	}

	if err := traefik.EnsureNetwork(t.Context(), m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.runtimeCalls) != 1 {
		t.Fatalf("expected 1 Runtime call, got %d", len(m.runtimeCalls))
	}
	got := strings.Join(m.runtimeCalls[0], " ")
	if got != "network create tug" {
		t.Errorf("expected 'network create tug', got %q", got)
	}
}

func TestEnsureNetwork_CreateFails(t *testing.T) {
	t.Parallel()

	m := &mockRunner{
		runtimeOutputFunc: func(_ []string) ([]byte, error) {
			return nil, errors.New("No such network: tug")
		},
		runtimeFunc: func(_ []string) error {
			return errors.New("permission denied")
		},
	}

	err := traefik.EnsureNetwork(t.Context(), m)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied in error, got %v", err)
	}
}

func TestEnsureNetwork_InspectError(t *testing.T) {
	t.Parallel()

	m := &mockRunner{
		runtimeOutputFunc: func(_ []string) ([]byte, error) {
			// Docker daemon unavailable — not a "not found" error
			return nil, errors.New("Cannot connect to the Docker daemon")
		},
	}

	err := traefik.EnsureNetwork(t.Context(), m)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "inspecting network") {
		t.Errorf("expected 'inspecting network' in error, got %v", err)
	}
	if len(m.runtimeCalls) != 0 {
		t.Errorf("expected no Runtime calls (should not attempt create), got %d", len(m.runtimeCalls))
	}
}

func TestEnsureRunning_AlreadyRunning(t *testing.T) {
	t.Parallel()

	call := 0
	m := &mockRunner{
		runtimeOutputFunc: func(args []string) ([]byte, error) {
			call++
			if call == 1 {
				// EnsureNetwork: network inspect succeeds
				return []byte("{}"), nil
			}
			// Container inspect: running
			return []byte("true\n"), nil
		},
	}

	if err := traefik.EnsureRunning(t.Context(), m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.runtimeCalls) != 0 {
		t.Errorf("expected no Runtime calls, got %d: %v", len(m.runtimeCalls), m.runtimeCalls)
	}
}

func TestEnsureRunning_ContainerNotExist_Starts(t *testing.T) {
	t.Parallel()

	call := 0
	m := &mockRunner{
		runtimeOutputFunc: func(args []string) ([]byte, error) {
			call++
			if call == 1 {
				// EnsureNetwork: network exists
				return []byte("{}"), nil
			}
			// Container inspect: not found
			return nil, errors.New("no such container")
		},
	}

	if err := traefik.EnsureRunning(t.Context(), m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only call "run" (no "rm" since inspect failed)
	if len(m.runtimeCalls) != 1 {
		t.Fatalf("expected 1 Runtime call, got %d: %v", len(m.runtimeCalls), m.runtimeCalls)
	}
	if m.runtimeCalls[0][0] != "run" {
		t.Errorf("expected 'run' command, got %q", m.runtimeCalls[0][0])
	}
}

func TestEnsureRunning_StoppedContainer_RemovesAndStarts(t *testing.T) {
	t.Parallel()

	call := 0
	m := &mockRunner{
		runtimeOutputFunc: func(args []string) ([]byte, error) {
			call++
			if call == 1 {
				// EnsureNetwork: network exists
				return []byte("{}"), nil
			}
			// Container inspect: exists but stopped
			return []byte("false\n"), nil
		},
	}

	if err := traefik.EnsureRunning(t.Context(), m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should call "rm" then "run"
	if len(m.runtimeCalls) != 2 {
		t.Fatalf("expected 2 Runtime calls, got %d: %v", len(m.runtimeCalls), m.runtimeCalls)
	}
	if m.runtimeCalls[0][0] != "rm" {
		t.Errorf("expected 'rm' as first call, got %q", m.runtimeCalls[0][0])
	}
	if m.runtimeCalls[1][0] != "run" {
		t.Errorf("expected 'run' as second call, got %q", m.runtimeCalls[1][0])
	}
}

func TestEnsureRunning_StartFails(t *testing.T) {
	t.Parallel()

	call := 0
	m := &mockRunner{
		runtimeOutputFunc: func(_ []string) ([]byte, error) {
			call++
			if call == 1 {
				return []byte("{}"), nil
			}
			return nil, errors.New("no such container")
		},
		runtimeFunc: func(args []string) error {
			if args[0] == "run" {
				return errors.New("image pull failed")
			}
			return nil
		},
	}

	err := traefik.EnsureRunning(t.Context(), m)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "image pull failed") {
		t.Errorf("expected 'image pull failed' in error, got %v", err)
	}
}

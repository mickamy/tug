package traefik_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickamy/tug/internal/config"
	"github.com/mickamy/tug/internal/traefik"
)

// mockRunner records calls and returns preconfigured responses.
type mockRunner struct {
	runtimeCalls       [][]string
	runtimeSilentCalls [][]string
	runtimeOutputCalls [][]string

	runtimeFunc       func(args []string) error
	runtimeSilentFunc func(args []string) ([]byte, error)
	runtimeOutputFunc func(args []string) ([]byte, error)
}

func (m *mockRunner) Runtime(_ context.Context, args ...string) error {
	m.runtimeCalls = append(m.runtimeCalls, args)
	if m.runtimeFunc != nil {
		return m.runtimeFunc(args)
	}
	return nil
}

func (m *mockRunner) RuntimeSilent(_ context.Context, args ...string) ([]byte, error) {
	m.runtimeSilentCalls = append(m.runtimeSilentCalls, args)
	if m.runtimeSilentFunc != nil {
		return m.runtimeSilentFunc(args)
	}
	return nil, nil
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
		runtimeOutputFunc: func(_ []string) ([]byte, error) {
			// "network inspect tug" fails → network does not exist
			return nil, errors.New("No such network: tug")
		},
	}

	if err := traefik.EnsureNetwork(t.Context(), m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.runtimeSilentCalls) != 1 {
		t.Fatalf("expected 1 RuntimeSilent call, got %d", len(m.runtimeSilentCalls))
	}
	got := strings.Join(m.runtimeSilentCalls[0], " ")
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
		runtimeSilentFunc: func(_ []string) ([]byte, error) {
			return nil, errors.New("permission denied")
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

	if err := traefik.EnsureRunning(t.Context(), m, config.Traefik{Port: 80}); err != nil {
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
			switch call {
			case 1:
				// EnsureNetwork: network exists
				return []byte("{}"), nil
			default:
				// Container inspect: not found
				return nil, errors.New("no such container")
			}
		},
	}

	if err := traefik.EnsureRunning(t.Context(), m, config.Traefik{Port: 80}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "run" goes through RuntimeSilent
	if len(m.runtimeSilentCalls) != 1 {
		t.Fatalf("expected 1 RuntimeSilent call, got %d: %v", len(m.runtimeSilentCalls), m.runtimeSilentCalls)
	}
	if m.runtimeSilentCalls[0][0] != "run" {
		t.Errorf("expected 'run' command, got %q", m.runtimeSilentCalls[0][0])
	}
}

func TestEnsureRunning_StoppedContainer_RemovesAndStarts(t *testing.T) {
	t.Parallel()

	call := 0
	m := &mockRunner{
		runtimeOutputFunc: func(args []string) ([]byte, error) {
			call++
			switch call {
			case 1:
				// EnsureNetwork: network exists
				return []byte("{}"), nil
			default:
				// Container inspect: exists but stopped
				return []byte("false\n"), nil
			}
		},
	}

	if err := traefik.EnsureRunning(t.Context(), m, config.Traefik{Port: 80}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "rm" via RuntimeOutput, "run" via RuntimeSilent
	if len(m.runtimeOutputCalls) != 3 {
		t.Fatalf("expected 3 RuntimeOutput calls, got %d: %v", len(m.runtimeOutputCalls), m.runtimeOutputCalls)
	}
	if m.runtimeOutputCalls[2][0] != "rm" {
		t.Errorf("expected 'rm' as 3rd RuntimeOutput call, got %q", m.runtimeOutputCalls[2][0])
	}
	if len(m.runtimeSilentCalls) != 1 {
		t.Fatalf("expected 1 RuntimeSilent call, got %d: %v", len(m.runtimeSilentCalls), m.runtimeSilentCalls)
	}
	if m.runtimeSilentCalls[0][0] != "run" {
		t.Errorf("expected 'run' as RuntimeSilent call, got %q", m.runtimeSilentCalls[0][0])
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
		runtimeSilentFunc: func(_ []string) ([]byte, error) {
			return nil, errors.New("image pull failed")
		},
	}

	err := traefik.EnsureRunning(t.Context(), m, config.Traefik{Port: 80})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "image pull failed") {
		t.Errorf("expected 'image pull failed' in error, got %v", err)
	}
}

func TestEnsureRunning_CustomPort(t *testing.T) {
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
	}

	if err := traefik.EnsureRunning(t.Context(), m, config.Traefik{Port: 8080}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.runtimeSilentCalls) != 1 {
		t.Fatalf("expected 1 RuntimeSilent call, got %d: %v", len(m.runtimeSilentCalls), m.runtimeSilentCalls)
	}
	args := strings.Join(m.runtimeSilentCalls[0], " ")
	if !strings.Contains(args, "127.0.0.1:8080:80") {
		t.Errorf("expected port binding '127.0.0.1:8080:80' in args, got %q", args)
	}
}

func TestEnsureRunning_DashboardEnabled(t *testing.T) {
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
	}

	if err := traefik.EnsureRunning(t.Context(), m, config.Traefik{Port: 80, Dashboard: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.runtimeSilentCalls) != 1 {
		t.Fatalf("expected 1 RuntimeSilent call, got %d: %v", len(m.runtimeSilentCalls), m.runtimeSilentCalls)
	}
	args := strings.Join(m.runtimeSilentCalls[0], " ")
	if !strings.Contains(args, "--api.insecure=true") {
		t.Errorf("expected '--api.insecure=true' in args, got %q", args)
	}
}

func TestEnsureRunning_DashboardDisabled(t *testing.T) {
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
	}

	if err := traefik.EnsureRunning(t.Context(), m, config.Traefik{Port: 80, Dashboard: false}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.runtimeSilentCalls) != 1 {
		t.Fatalf("expected 1 RuntimeSilent call, got %d: %v", len(m.runtimeSilentCalls), m.runtimeSilentCalls)
	}
	args := strings.Join(m.runtimeSilentCalls[0], " ")
	if strings.Contains(args, "--api.insecure") {
		t.Errorf("expected no '--api.insecure' in args, got %q", args)
	}
}

func TestStop_ContainerMissing(t *testing.T) {
	t.Parallel()

	m := &mockRunner{
		runtimeOutputFunc: func(_ []string) ([]byte, error) {
			return nil, errors.New("No such container: tug-traefik")
		},
	}

	if err := traefik.Stop(t.Context(), m); err != nil {
		t.Fatalf("expected no error for missing container, got %v", err)
	}
	// rm (fails with "No such container") + network rm = 2 RuntimeOutput calls.
	if len(m.runtimeOutputCalls) != 2 {
		t.Fatalf("expected 2 RuntimeOutput calls, got %d", len(m.runtimeOutputCalls))
	}
	if m.runtimeOutputCalls[1][0] != "network" {
		t.Errorf("expected 'network' command, got %q", m.runtimeOutputCalls[1][0])
	}
}

func TestStop_ContainerExists(t *testing.T) {
	t.Parallel()

	m := &mockRunner{
		runtimeOutputFunc: func(_ []string) ([]byte, error) {
			return []byte("tug-traefik\n"), nil
		},
	}

	if err := traefik.Stop(t.Context(), m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// rm + network rm = 2 RuntimeOutput calls.
	if len(m.runtimeOutputCalls) != 2 {
		t.Fatalf("expected 2 RuntimeOutput calls, got %d", len(m.runtimeOutputCalls))
	}
}

func TestStop_RmError(t *testing.T) {
	t.Parallel()

	m := &mockRunner{
		runtimeOutputFunc: func(_ []string) ([]byte, error) {
			return nil, errors.New("Cannot connect to the Docker daemon")
		},
	}

	err := traefik.Stop(t.Context(), m)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "removing traefik container") {
		t.Errorf("expected 'removing traefik container' in error, got %v", err)
	}
	// Should not attempt network removal when rm fails (only 1 RuntimeOutput call: rm).
	if len(m.runtimeOutputCalls) != 1 {
		t.Errorf("expected 1 RuntimeOutput call, got %d", len(m.runtimeOutputCalls))
	}
}

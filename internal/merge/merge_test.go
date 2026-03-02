package merge_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mickamy/tug/internal/merge"
)

type mockRunner struct {
	composeOutputArgs []string
	composeOutputData []byte
	composeOutputErr  error
}

func (m *mockRunner) Compose(_ context.Context, _ ...string) error                 { return nil }
func (m *mockRunner) Runtime(_ context.Context, _ ...string) error                 { return nil }
func (m *mockRunner) RuntimeSilent(_ context.Context, _ ...string) ([]byte, error) { return nil, nil }
func (m *mockRunner) RuntimeOutput(_ context.Context, _ ...string) ([]byte, error) { return nil, nil }
func (m *mockRunner) ComposeOutput(_ context.Context, args ...string) ([]byte, error) {
	m.composeOutputArgs = args
	return m.composeOutputData, m.composeOutputErr
}

func TestCompose(t *testing.T) {
	t.Parallel()

	body := []byte("name: myapp\nservices: {}\n")
	m := &mockRunner{composeOutputData: body}

	out, err := merge.Compose(t.Context(), m, "compose.yaml", "override.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"-f", "compose.yaml", "-f", "override.yaml", "config"}
	if !slices.Equal(m.composeOutputArgs, want) {
		t.Errorf("args: got %v, want %v", m.composeOutputArgs, want)
	}
	if string(out) != string(body) {
		t.Errorf("output: got %q, want %q", out, body)
	}
}

func TestCompose_Error(t *testing.T) {
	t.Parallel()

	m := &mockRunner{composeOutputErr: errors.New("config failed")}

	_, err := merge.Compose(t.Context(), m)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

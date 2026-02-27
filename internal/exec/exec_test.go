package exec_test

import (
	"testing"

	"github.com/mickamy/tug/internal/config"
	"github.com/mickamy/tug/internal/exec"
)

func TestCompose_EmptyCommand(t *testing.T) {
	t.Parallel()

	r := exec.New(config.Config{Command: config.Command{Compose: ""}})
	err := r.Compose(t.Context())
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestComposeOutput_RunsCommand(t *testing.T) {
	t.Parallel()

	r := exec.New(config.Config{Command: config.Command{Compose: "echo hello"}})
	out, err := r.ComposeOutput(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(out); got != "hello\n" {
		t.Errorf("got %q, want %q", got, "hello\n")
	}
}

func TestComposeOutput_MultiWordCommand(t *testing.T) {
	t.Parallel()

	r := exec.New(config.Config{Command: config.Command{Compose: "echo -n hi"}})
	out, err := r.ComposeOutput(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(out); got != "hi" {
		t.Errorf("got %q, want %q", got, "hi")
	}
}

func TestRuntimeOutput_RunsCommand(t *testing.T) {
	t.Parallel()

	r := exec.New(config.Config{Command: config.Command{Runtime: "echo"}})
	out, err := r.RuntimeOutput(t.Context(), "world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(out); got != "world\n" {
		t.Errorf("got %q, want %q", got, "world\n")
	}
}

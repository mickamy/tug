package exec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mickamy/tug/internal/config"
)

var errEmptyCommand = errors.New("empty command")

// Runner abstracts compose and runtime command execution.
type Runner interface {
	Compose(ctx context.Context, args ...string) error
	Runtime(ctx context.Context, args ...string) error
	RuntimeSilent(ctx context.Context, args ...string) ([]byte, error)
	RuntimeOutput(ctx context.Context, args ...string) ([]byte, error)
	ComposeOutput(ctx context.Context, args ...string) ([]byte, error)
}

type runner struct {
	cfg config.Config
}

// New creates a Runner from the given config.
func New(cfg config.Config) Runner {
	return &runner{cfg: cfg}
}

// Compose runs a compose command (e.g. "docker compose up") with the given
// args, inheriting stdin/stdout/stderr.
func (r *runner) Compose(ctx context.Context, args ...string) error {
	return r.run(ctx, r.cfg.Command.Compose, args)
}

// Runtime runs a runtime command (e.g. "docker run") with the given args,
// inheriting stdin/stdout/stderr.
func (r *runner) Runtime(ctx context.Context, args ...string) error {
	return r.run(ctx, r.cfg.Command.Runtime, args)
}

// RuntimeSilent runs a runtime command, suppressing stdout while streaming
// stderr to the terminal. Returns captured stdout so callers can inspect it on
// error. Use this instead of RuntimeOutput when the command may produce
// progress output on stdout (e.g. image pulls).
func (r *runner) RuntimeSilent(ctx context.Context, args ...string) ([]byte, error) {
	return r.silent(ctx, r.cfg.Command.Runtime, args)
}

// RuntimeOutput runs a runtime command and returns its combined output.
func (r *runner) RuntimeOutput(ctx context.Context, args ...string) ([]byte, error) {
	return r.output(ctx, r.cfg.Command.Runtime, args)
}

// ComposeOutput runs a compose command and returns its combined output.
func (r *runner) ComposeOutput(ctx context.Context, args ...string) ([]byte, error) {
	return r.output(ctx, r.cfg.Command.Compose, args)
}

func buildArgs(command string) (string, []string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil, errEmptyCommand
	}
	return parts[0], parts[1:], nil
}

func (r *runner) run(ctx context.Context, command string, args []string) error {
	bin, baseArgs, err := buildArgs(command)
	if err != nil {
		return err
	}
	cmdArgs := make([]string, 0, len(baseArgs)+len(args))
	cmdArgs = append(cmdArgs, baseArgs...)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, bin, cmdArgs...) //nolint:gosec // command comes from user config, not untrusted input
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec %s: %w", bin, err)
	}
	return nil
}

func (r *runner) silent(ctx context.Context, command string, args []string) ([]byte, error) {
	bin, baseArgs, err := buildArgs(command)
	if err != nil {
		return nil, err
	}
	cmdArgs := make([]string, 0, len(baseArgs)+len(args))
	cmdArgs = append(cmdArgs, baseArgs...)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, bin, cmdArgs...) //nolint:gosec // command comes from user config, not untrusted input
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("exec %s: %w", bin, err)
	}
	return out, nil
}

func (r *runner) output(ctx context.Context, command string, args []string) ([]byte, error) {
	bin, baseArgs, err := buildArgs(command)
	if err != nil {
		return nil, err
	}
	cmdArgs := make([]string, 0, len(baseArgs)+len(args))
	cmdArgs = append(cmdArgs, baseArgs...)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, bin, cmdArgs...) //nolint:gosec // command comes from user config, not untrusted input
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("exec %s: %w", bin, err)
	}
	return out, nil
}

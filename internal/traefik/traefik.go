package traefik

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickamy/tug/internal/config"
	"github.com/mickamy/tug/internal/exec"
)

const (
	containerName = "tug-traefik"
	networkName   = "tug"
	traefikImage  = "traefik:v3"
)

// NetworkName returns the Docker network name used by tug.
func NetworkName() string {
	return networkName
}

// EnsureNetwork creates the tug Docker network if it does not exist.
func EnsureNetwork(ctx context.Context, runner exec.Runner) error {
	out, err := runner.RuntimeOutput(ctx, "network", "inspect", networkName)
	if err == nil {
		return nil
	}
	// Distinguish "not found" from other failures (e.g. Docker unavailable).
	// Check both the error and combined output, as the message location varies by Docker version.
	msg := err.Error() + " " + string(out)
	if !strings.Contains(msg, "No such network") && !strings.Contains(msg, "not found") {
		return fmt.Errorf("inspecting network: %w", err)
	}
	if err := runner.Runtime(ctx, "network", "create", networkName); err != nil {
		return fmt.Errorf("creating network: %w", err)
	}
	return nil
}

// EnsureRunning starts the tug-traefik container if it is not already running.
func EnsureRunning(ctx context.Context, runner exec.Runner, cfg config.Traefik) error {
	if err := EnsureNetwork(ctx, runner); err != nil {
		return fmt.Errorf("ensuring network: %w", err)
	}

	out, err := runner.RuntimeOutput(ctx,
		"inspect", "-f", "{{.State.Running}}",
		containerName,
	)
	if err == nil && strings.TrimSpace(string(out)) == "true" {
		return nil
	}

	// Remove stopped/dead container if it exists (inspect succeeded but not running).
	if err == nil {
		_ = runner.Runtime(ctx, "rm", "-f", containerName)
	}

	runArgs := []string{
		"run", "-d",
		"--name", containerName,
		"--network", networkName,
		"--restart=unless-stopped",
		"-p", fmt.Sprintf("127.0.0.1:%d:80", cfg.Port),
		"-v", "/var/run/docker.sock:/var/run/docker.sock:ro",
		traefikImage,
	}
	if cfg.Dashboard {
		runArgs = append(runArgs, "--api.insecure=true")
	}
	runArgs = append(runArgs,
		"--providers.docker=true",
		"--providers.docker.exposedByDefault=false",
		"--providers.docker.network="+networkName,
	)

	if err := runner.Runtime(ctx, runArgs...); err != nil {
		return fmt.Errorf("starting traefik: %w", err)
	}
	return nil
}

// Stop removes the tug-traefik container and the tug network.
// It is idempotent — calling it when resources do not exist is not an error.
// Network removal failures are always ignored (other containers may be attached).
func Stop(ctx context.Context, runner exec.Runner) error {
	out, err := runner.RuntimeOutput(ctx, "rm", "-f", containerName)
	if err != nil {
		// "No such container" is expected when already removed; ignore.
		// Check both the error and combined output, as the message location varies by Docker version.
		msg := err.Error() + " " + string(out)
		if !strings.Contains(msg, "No such container") {
			return fmt.Errorf("removing traefik container: %w", err)
		}
	}
	// Network removal may fail if other containers are still attached; ignore.
	_ = runner.Runtime(ctx, "network", "rm", networkName)
	return nil
}

package traefik

import (
	"bytes"
	"context"
	"fmt"
	"strings"

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
	out, _ := runner.RuntimeOutput(ctx,
		"network", "ls", "--filter", "name=^"+networkName+"$", "--format", "{{.Name}}",
	)
	if strings.TrimSpace(string(out)) == networkName {
		return nil
	}
	if err := runner.Runtime(ctx, "network", "create", networkName); err != nil {
		return fmt.Errorf("creating network: %w", err)
	}
	return nil
}

// EnsureRunning starts the tug-traefik container if it is not already running.
func EnsureRunning(ctx context.Context, runner exec.Runner) error {
	if err := EnsureNetwork(ctx, runner); err != nil {
		return fmt.Errorf("ensuring network: %w", err)
	}

	out, _ := runner.RuntimeOutput(ctx,
		"inspect", "-f", "{{.State.Running}}",
		containerName,
	)
	if strings.TrimSpace(string(out)) == "true" {
		return nil
	}

	// Remove stopped container if it exists.
	existsOut, _ := runner.RuntimeOutput(ctx,
		"inspect", "-f", "{{.Name}}",
		containerName,
	)
	if len(bytes.TrimSpace(existsOut)) > 0 {
		_ = runner.Runtime(ctx, "rm", "-f", containerName)
	}

	if err := runner.Runtime(ctx,
		"run", "-d",
		"--name", containerName,
		"--network", networkName,
		"--restart=unless-stopped",
		"-p", "80:80",
		"-v", "/var/run/docker.sock:/var/run/docker.sock:ro",
		traefikImage,
		"--api.insecure=true",
		"--providers.docker=true",
		"--providers.docker.exposedByDefault=false",
		"--providers.docker.network="+networkName,
	); err != nil {
		return fmt.Errorf("starting traefik: %w", err)
	}
	return nil
}

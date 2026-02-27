package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickamy/tug/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg, err := config.Load(dir, filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Command.Compose != config.DefaultComposeCommand {
		t.Errorf("compose: got %q, want %q", cfg.Command.Compose, config.DefaultComposeCommand)
	}
	if cfg.Command.Runtime != config.DefaultRuntimeCommand {
		t.Errorf("runtime: got %q, want %q", cfg.Command.Runtime, config.DefaultRuntimeCommand)
	}
}

func TestLoad_LocalOverridesGlobal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	globalDir := filepath.Join(dir, "global")
	if err := os.MkdirAll(globalDir, 0o750); err != nil {
		t.Fatal(err)
	}
	globalPath := filepath.Join(globalDir, "tug.yaml")
	if err := os.WriteFile(
		globalPath,
		[]byte("command:\n  compose: \"global-compose\"\n  runtime: \"global-runtime\"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	localDir := filepath.Join(dir, "project")
	if err := os.MkdirAll(localDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(localDir, "tug.yaml"),
		[]byte("command:\n  compose: \"local-compose\"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(localDir, globalPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Command.Compose != "local-compose" {
		t.Errorf("compose: got %q, want %q", cfg.Command.Compose, "local-compose")
	}
	if cfg.Command.Runtime != "global-runtime" {
		t.Errorf("runtime: got %q, want %q", cfg.Command.Runtime, "global-runtime")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, "tug.yaml"),
		[]byte(":\nbad yaml [[["),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(dir, filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

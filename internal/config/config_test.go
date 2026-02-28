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
	if cfg.Traefik.Port != config.DefaultTraefikPort {
		t.Errorf("traefik port: got %d, want %d", cfg.Traefik.Port, config.DefaultTraefikPort)
	}
	if cfg.Traefik.Dashboard {
		t.Error("traefik dashboard: got true, want false")
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
		filepath.Join(localDir, ".tug.yaml"),
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

func TestLoad_TraefikLocalOverridesGlobal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	globalDir := filepath.Join(dir, "global")
	if err := os.MkdirAll(globalDir, 0o750); err != nil {
		t.Fatal(err)
	}
	globalPath := filepath.Join(globalDir, "tug.yaml")
	if err := os.WriteFile(
		globalPath,
		[]byte("traefik:\n  port: 8080\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	localDir := filepath.Join(dir, "project")
	if err := os.MkdirAll(localDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(localDir, ".tug.yaml"),
		[]byte("traefik:\n  port: 9090\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(localDir, globalPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Traefik.Port != 9090 {
		t.Errorf("traefik port: got %d, want 9090", cfg.Traefik.Port)
	}
}

func TestLoad_TraefikDashboardOptIn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, ".tug.yaml"),
		[]byte("traefik:\n  dashboard: true\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir, filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Traefik.Dashboard {
		t.Error("traefik dashboard: got false, want true")
	}
	if cfg.Traefik.Port != config.DefaultTraefikPort {
		t.Errorf("traefik port: got %d, want %d", cfg.Traefik.Port, config.DefaultTraefikPort)
	}
}

func TestLoad_ServiceOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	globalDir := filepath.Join(dir, "global")
	if err := os.MkdirAll(globalDir, 0o750); err != nil {
		t.Fatal(err)
	}
	globalPath := filepath.Join(globalDir, "tug.yaml")
	if err := os.WriteFile(
		globalPath,
		[]byte("services:\n  db:\n    kind: tcp\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	localDir := filepath.Join(dir, "project")
	if err := os.MkdirAll(localDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// Local overrides db to http, adds api as http.
	if err := os.WriteFile(
		filepath.Join(localDir, ".tug.yaml"),
		[]byte("services:\n  db:\n    kind: http\n  api:\n    kind: http\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(localDir, globalPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["db"].Kind != "http" {
		t.Errorf("db kind: got %q, want %q", cfg.Services["db"].Kind, "http")
	}
	if cfg.Services["api"].Kind != "http" {
		t.Errorf("api kind: got %q, want %q", cfg.Services["api"].Kind, "http")
	}
}

func TestLoad_ServiceConfigMerge(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.yaml")
	// Global: db has kind=tcp and port overrides.
	if err := os.WriteFile(
		globalPath,
		[]byte("services:\n  db:\n    kind: tcp\n    ports:\n      5432: tcp\n      8080: http\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	localDir := filepath.Join(dir, "project")
	if err := os.MkdirAll(localDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// Local: db overrides kind only; ports should be preserved from global.
	if err := os.WriteFile(
		filepath.Join(localDir, ".tug.yaml"),
		[]byte("services:\n  db:\n    kind: http\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(localDir, globalPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["db"].Kind != "http" {
		t.Errorf("kind: got %q, want %q", cfg.Services["db"].Kind, "http")
	}
	if cfg.Services["db"].Ports[5432] != "tcp" {
		t.Errorf("port 5432: got %q, want %q", cfg.Services["db"].Ports[5432], "tcp")
	}
	if cfg.Services["db"].Ports[8080] != "http" {
		t.Errorf("port 8080: got %q, want %q", cfg.Services["db"].Ports[8080], "http")
	}
}

func TestLoad_PortOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, ".tug.yaml"),
		[]byte("services:\n  sql-tap:\n    ports:\n      8081: http\n      9091: tcp\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir, filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sc := cfg.Services["sql-tap"]
	if sc.Ports[8081] != "http" {
		t.Errorf("port 8081: got %q, want %q", sc.Ports[8081], "http")
	}
	if sc.Ports[9091] != "tcp" {
		t.Errorf("port 9091: got %q, want %q", sc.Ports[9091], "tcp")
	}
}

func TestLoad_PortOverrideWithDefaultKind(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, ".tug.yaml"),
		[]byte("services:\n  proxy:\n    kind: tcp\n    ports:\n      8080: http\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir, filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sc := cfg.Services["proxy"]
	if sc.Kind != "tcp" {
		t.Errorf("kind: got %q, want %q", sc.Kind, "tcp")
	}
	if sc.Ports[8080] != "http" {
		t.Errorf("port 8080: got %q, want %q", sc.Ports[8080], "http")
	}
}

func TestLoad_InvalidPortKind(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, ".tug.yaml"),
		[]byte("services:\n  svc:\n    ports:\n      8080: udp\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(dir, filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Fatal("expected error for invalid port kind, got nil")
	}
}

func TestLoad_InvalidKind(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, ".tug.yaml"),
		[]byte("services:\n  db:\n    kind: udp\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(dir, filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Fatal("expected error for invalid kind, got nil")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(dir, ".tug.yaml"),
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

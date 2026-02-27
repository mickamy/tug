package compose_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickamy/tug/internal/compose"
)

func TestFindComposeFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
	}{
		{"compose.yaml", "compose.yaml"},
		{"compose.yml", "compose.yml"},
		{"docker-compose.yaml", "docker-compose.yaml"},
		{"docker-compose.yml", "docker-compose.yml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(path, []byte("services: {}"), 0o600); err != nil {
				t.Fatal(err)
			}

			got, err := compose.FindComposeFile(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != path {
				t.Errorf("got %q, want %q", got, path)
			}
		})
	}
}

func TestFindComposeFile_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := compose.FindComposeFile(dir)
	if err == nil {
		t.Fatal("expected error when no compose file exists")
	}
}

func TestFindComposeFile_Priority(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create both; compose.yaml should win.
	for _, name := range []string{"compose.yaml", "docker-compose.yml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("services: {}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := compose.FindComposeFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "compose.yaml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	content := `name: myapp
services:
  api:
    image: node:20
    ports:
      - "3000:3000"
  postgres:
    image: postgres:16
    ports:
      - "127.0.0.1:5432:5432"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	proj, err := compose.Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proj.Name != "myapp" {
		t.Errorf("name: got %q, want %q", proj.Name, "myapp")
	}
	if len(proj.Services) != 2 {
		t.Fatalf("services: got %d, want 2", len(proj.Services))
	}

	svcMap := make(map[string]compose.Service)
	for _, s := range proj.Services {
		svcMap[s.Name] = s
	}

	api, ok := svcMap["api"]
	if !ok {
		t.Fatal("missing service: api")
	}
	if api.Image != "node:20" {
		t.Errorf("api image: got %q, want %q", api.Image, "node:20")
	}
	if len(api.Ports) != 1 || api.Ports[0].Host != 3000 || api.Ports[0].Container != 3000 {
		t.Errorf("api ports: got %+v, want [{Host:3000 Container:3000}]", api.Ports)
	}

	pg, ok := svcMap["postgres"]
	if !ok {
		t.Fatal("missing service: postgres")
	}
	if len(pg.Ports) != 1 || pg.Ports[0].Host != 5432 || pg.Ports[0].Container != 5432 {
		t.Errorf("postgres ports: got %+v, want [{Host:5432 Container:5432}]", pg.Ports)
	}
}

func TestParse_ContainerOnlyPort_Skipped(t *testing.T) {
	t.Parallel()

	content := `name: myapp
services:
  web:
    image: nginx
    ports:
      - "8080"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	proj, err := compose.Parse(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proj.Services) != 1 {
		t.Fatalf("services: got %d, want 1", len(proj.Services))
	}
	if len(proj.Services[0].Ports) != 0 {
		t.Errorf("ports: got %+v, want empty (container-only ports should be skipped)", proj.Services[0].Ports)
	}
}

func TestParse_InvalidPort(t *testing.T) {
	t.Parallel()

	content := `services:
  bad:
    ports:
      - "abc:def"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := compose.Parse(path)
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

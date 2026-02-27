package compose_test

import (
	"strings"
	"testing"

	"github.com/mickamy/tug/internal/compose"
)

func TestClassify_HTTP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "api", Image: "node:20", Ports: []compose.Port{{Host: 3000, Container: 3000}}},
		},
	}

	result := compose.Classify(proj)
	if len(result) != 1 {
		t.Fatalf("got %d services, want 1", len(result))
	}
	if result[0].Kind != compose.KindHTTP {
		t.Errorf("kind: got %d, want KindHTTP", result[0].Kind)
	}
}

func TestClassify_TCP_ByPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		containerPort uint16
	}{
		{"postgres", 5432},
		{"mysql", 3306},
		{"redis", 6379},
		{"mongo", 27017},
		{"memcached", 11211},
		{"rabbitmq", 5672},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proj := compose.Project{
				Name: "myapp",
				Services: []compose.Service{
					{Name: "db", Image: "whatever", Ports: []compose.Port{{Host: tt.containerPort, Container: tt.containerPort}}},
				},
			}

			result := compose.Classify(proj)
			if result[0].Kind != compose.KindTCP {
				t.Errorf("port %d: got KindHTTP, want KindTCP", tt.containerPort)
			}
			if result[0].HostPort < 10000 || result[0].HostPort >= 60000 {
				t.Errorf("host port %d out of range", result[0].HostPort)
			}
		})
	}
}

func TestClassify_TCP_CustomImage(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "db", Image: "ghcr.io/myorg/custom-db:v1", Ports: []compose.Port{{Host: 5432, Container: 5432}}},
		},
	}

	result := compose.Classify(proj)
	if result[0].Kind != compose.KindTCP {
		t.Error("custom image with well-known port should be KindTCP")
	}
}

func TestClassify_DeterministicPort(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "db", Image: "postgres:16", Ports: []compose.Port{{Host: 5432, Container: 5432}}},
		},
	}

	a := compose.Classify(proj)
	b := compose.Classify(proj)
	if a[0].HostPort != b[0].HostPort {
		t.Errorf("expected deterministic port, got %d and %d", a[0].HostPort, b[0].HostPort)
	}
}

func TestGenerateOverride_HTTP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "api", Image: "node:20", Ports: []compose.Port{{Host: 3000, Container: 3000}}},
		},
	}
	classified := compose.Classify(proj)

	data, err := compose.GenerateOverride(proj, classified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "traefik.enable=true") {
		t.Error("missing traefik.enable label")
	}
	if !strings.Contains(out, "api.myapp.localhost") {
		t.Error("missing host rule")
	}
	if !strings.Contains(out, "tug") {
		t.Error("missing tug network")
	}
}

func TestGenerateOverride_TCP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "db", Image: "postgres:16", Ports: []compose.Port{{Host: 5432, Container: 5432}}},
		},
	}
	classified := compose.Classify(proj)

	data, err := compose.GenerateOverride(proj, classified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "target: 5432") {
		t.Error("missing target port")
	}
	if !strings.Contains(out, "protocol: tcp") {
		t.Error("missing protocol")
	}
}

package override_test

import (
	"strings"
	"testing"

	"github.com/mickamy/tug/internal/compose"
	"github.com/mickamy/tug/internal/config"
	"github.com/mickamy/tug/internal/override"
)

func TestClassify_HTTP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "api", Image: "node:20", Ports: []compose.Port{{Host: 3000, Container: 3000}}},
		},
	}

	result, err := override.Classify(proj, config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("got %d services, want 1", len(result))
	}
	if len(result[0].ClassifiedPorts) != 1 {
		t.Fatalf("got %d ports, want 1", len(result[0].ClassifiedPorts))
	}
	if result[0].ClassifiedPorts[0].Kind != override.KindHTTP {
		t.Errorf("kind: got %v, want KindHTTP", result[0].ClassifiedPorts[0].Kind)
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

			result, err := override.Classify(proj, config.Config{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			cp := result[0].ClassifiedPorts[0]
			if cp.Kind != override.KindTCP {
				t.Errorf("port %d: got KindHTTP, want KindTCP", tt.containerPort)
			}
			if cp.HostPort < 10000 || cp.HostPort >= 60000 {
				t.Errorf("host port %d out of range", cp.HostPort)
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

	result, err := override.Classify(proj, config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0].ClassifiedPorts[0].Kind != override.KindTCP {
		t.Error("custom image with well-known port should be KindTCP")
	}
}

func TestClassify_ConfigOverride_TCP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			// Port 9090 is not a well-known TCP port, but config says TCP.
			{Name: "custom-db", Image: "myimage", Ports: []compose.Port{{Host: 9090, Container: 9090}}},
		},
	}

	cfg := config.Config{
		Services: map[string]config.ServiceConfig{
			"custom-db": {Kind: "tcp"},
		},
	}

	result, err := override.Classify(proj, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cp := result[0].ClassifiedPorts[0]
	if cp.Kind != override.KindTCP {
		t.Error("config override to tcp should take effect")
	}
	if cp.ContainerPort != 9090 {
		t.Errorf("container port: got %d, want 9090", cp.ContainerPort)
	}
}

func TestClassify_ConfigOverride_HTTP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			// Port 5432 would normally be TCP, but config says HTTP.
			{Name: "pg-proxy", Image: "myproxy", Ports: []compose.Port{{Host: 5432, Container: 5432}}},
		},
	}

	cfg := config.Config{
		Services: map[string]config.ServiceConfig{
			"pg-proxy": {Kind: "http"},
		},
	}

	result, err := override.Classify(proj, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0].ClassifiedPorts[0].Kind != override.KindHTTP {
		t.Error("config override to http should take effect")
	}
}

func TestClassify_NoPorts_RespectsConfigKind(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "worker", Image: "worker:latest"},
		},
	}

	cfg := config.Config{
		Services: map[string]config.ServiceConfig{
			"worker": {Kind: "tcp"},
		},
	}

	result, err := override.Classify(proj, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0].ClassifiedPorts[0].Kind != override.KindTCP {
		t.Error("service with no ports and kind=tcp should be KindTCP")
	}
}

func TestClassify_PerPortOverride(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{
				Name:  "sql-tap",
				Image: "ghcr.io/mickamy/sql-tapd:latest",
				Ports: []compose.Port{
					{Host: 8081, Container: 8081},
					{Host: 9091, Container: 9091},
				},
			},
		},
	}

	cfg := config.Config{
		Services: map[string]config.ServiceConfig{
			"sql-tap": {
				Ports: map[uint16]string{
					8081: "http",
					9091: "tcp",
				},
			},
		},
	}

	result, err := override.Classify(proj, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cps := result[0].ClassifiedPorts
	if len(cps) != 2 {
		t.Fatalf("got %d ports, want 2", len(cps))
	}
	if cps[0].Kind != override.KindHTTP {
		t.Errorf("port 8081: got %v, want KindHTTP", cps[0].Kind)
	}
	if cps[1].Kind != override.KindTCP {
		t.Errorf("port 9091: got %v, want KindTCP", cps[1].Kind)
	}
	if cps[1].HostPort < 10000 || cps[1].HostPort >= 60000 {
		t.Errorf("host port %d out of range", cps[1].HostPort)
	}
}

func TestClassify_PerPortOverridesServiceKind(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{
				Name:  "proxy",
				Image: "myproxy",
				Ports: []compose.Port{
					{Host: 8080, Container: 8080},
					{Host: 5432, Container: 5432},
				},
			},
		},
	}

	cfg := config.Config{
		Services: map[string]config.ServiceConfig{
			"proxy": {
				Kind: "tcp", // default
				Ports: map[uint16]string{
					8080: "http", // override for this port
				},
			},
		},
	}

	result, err := override.Classify(proj, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cps := result[0].ClassifiedPorts
	if cps[0].Kind != override.KindHTTP {
		t.Errorf("port 8080: got %v, want KindHTTP (per-port override)", cps[0].Kind)
	}
	if cps[1].Kind != override.KindTCP {
		t.Errorf("port 5432: got %v, want KindTCP (service-level default)", cps[1].Kind)
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

	a, err := override.Classify(proj, config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := override.Classify(proj, config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a[0].ClassifiedPorts[0].HostPort != b[0].ClassifiedPorts[0].HostPort {
		t.Errorf("expected deterministic port, got %d and %d",
			a[0].ClassifiedPorts[0].HostPort, b[0].ClassifiedPorts[0].HostPort)
	}
}

func TestGenerate_HTTP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "api", Image: "node:20", Ports: []compose.Port{{Host: 3000, Container: 3000}}},
		},
	}
	classified, err := override.Classify(proj, config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := override.Generate(proj, classified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	for _, want := range []string{
		"traefik.enable=true",
		"api.myapp.localhost",
		"external: true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestGenerate_TCP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "db", Image: "postgres:16", Ports: []compose.Port{{Host: 5432, Container: 5432}}},
		},
	}
	classified, err := override.Classify(proj, config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := override.Generate(proj, classified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	for _, want := range []string{
		"!override",
		"target: 5432",
		"protocol: tcp",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestGenerate_MixedHTTPAndTCP(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{
				Name:  "sql-tap",
				Image: "ghcr.io/mickamy/sql-tapd:latest",
				Ports: []compose.Port{
					{Host: 8081, Container: 8081},
					{Host: 9091, Container: 9091},
				},
			},
		},
	}
	cfg := config.Config{
		Services: map[string]config.ServiceConfig{
			"sql-tap": {
				Ports: map[uint16]string{
					8081: "http",
					9091: "tcp",
				},
			},
		},
	}
	classified, err := override.Classify(proj, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := override.Generate(proj, classified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	// Should have Traefik labels for HTTP port.
	for _, want := range []string{
		"traefik.enable=true",
		"sql-tap.myapp.localhost",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	// Should have TCP port remapping.
	for _, want := range []string{
		"!override",
		"target: 9091",
		"protocol: tcp",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestGenerate_NetworkSection(t *testing.T) {
	t.Parallel()

	proj := compose.Project{
		Name: "myapp",
		Services: []compose.Service{
			{Name: "api", Image: "node:20"},
		},
	}
	classified, err := override.Classify(proj, config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := override.Generate(proj, classified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "networks:") {
		t.Errorf("missing networks section in output:\n%s", out)
	}
	if !strings.Contains(out, "external: true") {
		t.Errorf("missing external: true in output:\n%s", out)
	}
}

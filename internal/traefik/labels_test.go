package traefik_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickamy/tug/internal/traefik"
)

func TestLabels_WithPort(t *testing.T) {
	t.Parallel()

	labels := traefik.Labels("myapp", "api", 3000)

	want := []string{
		"traefik.enable=true",
		"traefik.http.routers.myapp-api.rule=Host(`api.myapp.localhost`)",
		"traefik.http.services.myapp-api.loadbalancer.server.port=3000",
	}

	for _, w := range want {
		if !slices.Contains(labels, w) {
			t.Errorf("missing label: %s", w)
		}
	}
}

func TestLabels_WithoutPort(t *testing.T) {
	t.Parallel()

	labels := traefik.Labels("myapp", "web", 0)

	if len(labels) != 2 {
		t.Errorf("got %d labels, want 2 (no load-balancer label)", len(labels))
	}

	for _, l := range labels {
		if strings.Contains(l, "loadbalancer") {
			t.Error("unexpected load-balancer label when port is 0")
		}
	}
}

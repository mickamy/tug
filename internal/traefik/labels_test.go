package traefik_test

import (
	"strings"
	"testing"

	"github.com/mickamy/tug/internal/traefik"
)

func TestLabels_WithPort(t *testing.T) {
	t.Parallel()

	labels := traefik.Labels("myapp", "api", 3000)

	want := map[string]bool{
		"traefik.enable=true": false,
		"traefik.http.routers.myapp-api.rule=Host(`api.myapp.localhost`)": false,
		"traefik.http.services.myapp-api.loadbalancer.server.port=3000":   false,
	}

	for _, l := range labels {
		for k := range want {
			if strings.Contains(l, k) || l == k {
				want[k] = true
			}
		}
	}

	for k, found := range want {
		if !found {
			t.Errorf("missing label: %s", k)
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

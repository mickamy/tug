package compose

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/mickamy/tug/internal/port"
	"github.com/mickamy/tug/internal/traefik"
)

// tcpPorts maps well-known container ports to their protocol.
// Based on IANA standard port assignments.
var tcpPorts = map[uint16]struct{}{
	3306:  {}, // mysql, mariadb
	5432:  {}, // postgres
	5672:  {}, // rabbitmq
	6379:  {}, // redis, valkey
	11211: {}, // memcached
	27017: {}, // mongo
}

// ServiceKind indicates how tug should handle a service.
type ServiceKind int

const (
	KindHTTP ServiceKind = iota
	KindTCP
)

// ClassifiedService holds a service with its classification and resolved port info.
type ClassifiedService struct {
	Service

	Kind          ServiceKind
	HostPort      uint16 // assigned host port for TCP services
	ContainerPort uint16 // well-known container port for TCP services
}

// Classify categorizes each service as HTTP or TCP based on its container ports.
func Classify(proj Project) []ClassifiedService {
	used := make(map[uint16]struct{})
	res := make([]ClassifiedService, len(proj.Services))

	for i, svc := range proj.Services {
		kind, cp := detectKind(svc)
		res[i] = ClassifiedService{
			Service:       svc,
			Kind:          kind,
			ContainerPort: cp,
		}
		if kind == KindTCP && cp > 0 {
			hp := port.Compute(proj.Name, svc.Name, cp, used)
			res[i].HostPort = hp
			used[hp] = struct{}{}
		}
	}

	return res
}

// detectKind checks whether any of the service's container ports is a well-known TCP port.
func detectKind(svc Service) (ServiceKind, uint16) {
	for _, p := range svc.Ports {
		if _, ok := tcpPorts[p.Container]; ok {
			return KindTCP, p.Container
		}
	}
	return KindHTTP, 0
}

// GenerateOverride produces the override YAML for the given project and classified services.
// HTTP services get Traefik labels; TCP services get deterministic port remapping.
func GenerateOverride(proj Project, services []ClassifiedService) ([]byte, error) {
	override := map[string]any{
		"services": buildServices(proj.Name, services),
	}

	data, err := yaml.Marshal(override)
	if err != nil {
		return nil, fmt.Errorf("marshalling override: %w", err)
	}
	return data, nil
}

func buildServices(projectName string, services []ClassifiedService) map[string]any {
	svcMap := make(map[string]any, len(services))

	for _, cs := range services {
		switch cs.Kind {
		case KindHTTP:
			svcMap[cs.Name] = buildHTTPService(projectName, cs)
		case KindTCP:
			svcMap[cs.Name] = buildTCPService(cs)
		}
	}

	return svcMap
}

func buildHTTPService(projectName string, cs ClassifiedService) map[string]any {
	var containerPort uint16
	if len(cs.Ports) > 0 {
		containerPort = cs.Ports[0].Container
	}

	return map[string]any{
		"labels":   traefik.Labels(projectName, cs.Name, containerPort),
		"networks": []string{"tug"},
	}
}

func buildTCPService(cs ClassifiedService) map[string]any {
	if cs.HostPort == 0 || cs.ContainerPort == 0 {
		return map[string]any{}
	}

	return map[string]any{
		"ports": []map[string]any{
			{
				"target":    cs.ContainerPort,
				"published": cs.HostPort,
				"protocol":  "tcp",
				"mode":      "host",
			},
		},
	}
}

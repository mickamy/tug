package override

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/mickamy/tug/internal/compose"
	"github.com/mickamy/tug/internal/config"
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

func (k ServiceKind) String() string {
	switch k {
	case KindHTTP:
		return "http"
	case KindTCP:
		return "tcp"
	default:
		return fmt.Sprintf("ServiceKind(%d)", int(k))
	}
}

// ClassifiedService holds a service with its classification and resolved port info.
type ClassifiedService struct {
	compose.Service

	Kind          ServiceKind
	HostPort      uint16 // assigned host port for TCP services
	ContainerPort uint16 // well-known container port for TCP services
}

// Classify categorizes each service as HTTP or TCP.
// Priority: config override > well-known container port detection.
func Classify(proj compose.Project, cfg config.Config) ([]ClassifiedService, error) {
	used := make(map[uint16]struct{})
	res := make([]ClassifiedService, len(proj.Services))

	for i, svc := range proj.Services {
		kind, cp := detectKind(svc, cfg)
		res[i] = ClassifiedService{
			Service:       svc,
			Kind:          kind,
			ContainerPort: cp,
		}
		if kind == KindTCP && cp > 0 {
			hp, err := port.Compute(proj.Name, svc.Name, cp, used)
			if err != nil {
				return nil, fmt.Errorf("service %s: %w", svc.Name, err)
			}
			res[i].HostPort = hp
			used[hp] = struct{}{}
		}
	}

	return res, nil
}

// detectKind checks config overrides first, then falls back to well-known port detection.
func detectKind(svc compose.Service, cfg config.Config) (ServiceKind, uint16) {
	if sc, ok := cfg.Services[svc.Name]; ok {
		switch sc.Kind {
		case "tcp":
			if len(svc.Ports) > 0 {
				return KindTCP, svc.Ports[0].Container
			}
			return KindTCP, 0
		case "http":
			return KindHTTP, 0
		}
	}

	for _, p := range svc.Ports {
		if _, ok := tcpPorts[p.Container]; ok {
			return KindTCP, p.Container
		}
	}
	return KindHTTP, 0
}

// Generate produces the override YAML for the given project and classified services.
// HTTP services get Traefik labels + tug network; TCP services get deterministic port
// remapping with !override to replace (not append) the original ports.
func Generate(proj compose.Project, services []ClassifiedService) ([]byte, error) {
	root := map[string]any{
		"services": buildServices(proj.Name, services),
		"networks": map[string]any{
			traefik.NetworkName(): map[string]any{
				"external": true,
			},
		},
	}

	data, err := yaml.Marshal(root)
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
		"networks": []string{traefik.NetworkName()},
	}
}

func buildTCPService(cs ClassifiedService) map[string]any {
	if cs.HostPort == 0 || cs.ContainerPort == 0 {
		return map[string]any{}
	}

	return map[string]any{
		"ports": overrideSeq{
			{
				"target":    cs.ContainerPort,
				"published": cs.HostPort,
				"protocol":  "tcp",
				"mode":      "host",
			},
		},
	}
}

// overrideSeq marshals as a YAML sequence with the !override tag.
// Docker Compose v2.24+ uses this to fully replace (not merge) the original sequence.
type overrideSeq []map[string]any

func (s overrideSeq) MarshalYAML() (any, error) {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!override"}
	for _, item := range s {
		var n yaml.Node
		if err := n.Encode(item); err != nil {
			return nil, fmt.Errorf("encoding override item: %w", err)
		}
		seq.Content = append(seq.Content, &n)
	}
	return seq, nil
}

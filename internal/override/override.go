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

// ServiceKind indicates how tug should handle a port.
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

// ClassifiedPort holds a single port with its classification and resolved host port.
type ClassifiedPort struct {
	ContainerPort uint16
	Kind          ServiceKind
	HostPort      uint16 // assigned host port (TCP only)
}

// ClassifiedService holds a service with per-port classification.
type ClassifiedService struct {
	compose.Service

	ClassifiedPorts []ClassifiedPort
}

// Classify categorizes each port of each service as HTTP or TCP.
// Priority: config port override > config service-level kind > well-known port detection > HTTP default.
func Classify(proj compose.Project, cfg config.Config) ([]ClassifiedService, error) {
	used := make(map[uint16]struct{})
	res := make([]ClassifiedService, len(proj.Services))

	for i, svc := range proj.Services {
		cps := classifyPorts(svc, cfg)
		for j, cp := range cps {
			if cp.Kind == KindTCP && cp.ContainerPort > 0 {
				hp, err := port.Compute(proj.Name, svc.Name, cp.ContainerPort, used)
				if err != nil {
					return nil, fmt.Errorf("service %s port %d: %w", svc.Name, cp.ContainerPort, err)
				}
				cps[j].HostPort = hp
				used[hp] = struct{}{}
			}
		}
		res[i] = ClassifiedService{Service: svc, ClassifiedPorts: cps}
	}

	return res, nil
}

// classifyPorts determines the kind of each port in a service.
func classifyPorts(svc compose.Service, cfg config.Config) []ClassifiedPort {
	sc := cfg.Services[svc.Name]

	if len(svc.Ports) == 0 {
		return []ClassifiedPort{{Kind: KindHTTP}}
	}

	cps := make([]ClassifiedPort, len(svc.Ports))
	for i, p := range svc.Ports {
		cps[i] = ClassifiedPort{
			ContainerPort: p.Container,
			Kind:          detectPortKind(p.Container, sc),
		}
	}
	return cps
}

// detectPortKind determines the kind for a single port.
// Priority: config port override > config service kind > well-known port > HTTP default.
func detectPortKind(containerPort uint16, sc config.ServiceConfig) ServiceKind {
	if k, ok := sc.Ports[containerPort]; ok {
		if k == "tcp" {
			return KindTCP
		}
		return KindHTTP
	}

	if sc.Kind == "tcp" {
		return KindTCP
	}
	if sc.Kind == "http" {
		return KindHTTP
	}

	if _, ok := tcpPorts[containerPort]; ok {
		return KindTCP
	}
	return KindHTTP
}

// Generate produces the override YAML for the given project and classified services.
// HTTP ports get Traefik labels + tug network; TCP ports get deterministic port
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
		svcMap[cs.Name] = buildService(projectName, cs)
	}

	return svcMap
}

func buildService(projectName string, cs ClassifiedService) map[string]any {
	svc := map[string]any{}

	// Find the first HTTP port for Traefik routing.
	var httpPort uint16
	hasHTTP := false
	for _, cp := range cs.ClassifiedPorts {
		if cp.Kind == KindHTTP {
			httpPort = cp.ContainerPort
			hasHTTP = true
			break
		}
	}

	if hasHTTP {
		svc["labels"] = traefik.Labels(projectName, cs.Name, httpPort)
		svc["networks"] = []string{traefik.NetworkName()}
	}

	// Collect TCP port remappings.
	var tcpEntries overrideSeq
	for _, cp := range cs.ClassifiedPorts {
		if cp.Kind == KindTCP && cp.HostPort > 0 && cp.ContainerPort > 0 {
			tcpEntries = append(tcpEntries, map[string]any{
				"target":    cp.ContainerPort,
				"published": cp.HostPort,
				"protocol":  "tcp",
				"mode":      "host",
			})
		}
	}
	if len(tcpEntries) > 0 {
		svc["ports"] = tcpEntries
	}

	return svc
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

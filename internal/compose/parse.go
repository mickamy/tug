package compose

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// composeFilenames lists the filenames to search for, in priority order.
var composeFilenames = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

// Port represents a parsed port mapping from a compose file.
type Port struct {
	Host      uint16
	Container uint16
}

// Service represents a single service parsed from a compose file.
type Service struct {
	Name  string
	Image string
	Ports []Port
}

// Project represents a parsed compose project.
type Project struct {
	Name     string
	Services []Service
}

// composeFile is the minimal structure we need from compose YAML.
type composeFile struct {
	Name     string                    `yaml:"name"`
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image string      `yaml:"image"`
	Ports portEntries `yaml:"ports"`
}

// portEntries unmarshals both short ("8080:8080") and long
// ({target: 8080, published: "8080", …}) Docker Compose port syntax.
type portEntries []Port

func (pe *portEntries) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.SequenceNode {
		return errors.New("ports: expected sequence")
	}
	for _, node := range value.Content {
		p, ok, err := parsePortNode(node)
		if err != nil {
			return err
		}
		if ok {
			*pe = append(*pe, p)
		}
	}
	return nil
}

func parsePortNode(node *yaml.Node) (Port, bool, error) {
	switch node.Kind { //nolint:exhaustive // only scalar and mapping are valid port nodes
	case yaml.ScalarNode:
		return parseShortPort(node.Value)
	case yaml.MappingNode:
		return parseLongPort(node)
	default:
		return Port{}, false, fmt.Errorf("unexpected port node kind: %d", node.Kind)
	}
}

// FindComposeFile returns the path of the first compose file found in dir.
func FindComposeFile(dir string) (string, error) {
	for _, name := range composeFilenames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no compose file found in %s", dir)
}

// Parse reads a compose file and returns a Project.
func Parse(path string) (Project, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from FindComposeFile, not untrusted input
	if err != nil {
		return Project{}, fmt.Errorf("reading compose file: %w", err)
	}

	return ParseBytes(data)
}

// ParseBytes parses compose YAML from raw bytes.
func ParseBytes(data []byte) (Project, error) {
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return Project{}, fmt.Errorf("parsing compose file: %w", err)
	}

	proj := Project{Name: cf.Name}

	names := make([]string, 0, len(cf.Services))
	for name := range cf.Services {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		svc := cf.Services[name]
		proj.Services = append(proj.Services, Service{
			Name:  name,
			Image: svc.Image,
			Ports: svc.Ports,
		})
	}

	return proj, nil
}

// parseShortPort parses port strings in Docker Compose short syntax.
// Returns (port, true, nil) for mappings with a host port,
// or (Port{}, false, nil) for container-only ports (e.g. "8080") which tug skips.
func parseShortPort(raw string) (Port, bool, error) {
	parts := strings.Split(raw, ":")
	switch len(parts) {
	case 1:
		// "container" only — no host port to remap, skip
		return Port{}, false, nil
	case 2:
		// "host:container"
		p, err := parsePair(parts[0], parts[1])
		return p, err == nil, err
	case 3:
		// "ip:host:container"
		p, err := parsePair(parts[1], parts[2])
		return p, err == nil, err
	default:
		return Port{}, false, fmt.Errorf("invalid port format: %q", raw)
	}
}

func parsePair(hostStr, containerStr string) (Port, error) {
	host, err := strconv.ParseUint(stripProto(hostStr), 10, 16)
	if err != nil {
		return Port{}, fmt.Errorf("invalid host port %q: %w", hostStr, err)
	}
	container, err := strconv.ParseUint(stripProto(containerStr), 10, 16)
	if err != nil {
		return Port{}, fmt.Errorf("invalid container port %q: %w", containerStr, err)
	}
	return Port{Host: uint16(host), Container: uint16(container)}, nil
}

// stripProto removes an optional "/tcp" or "/udp" suffix from a port string.
func stripProto(s string) string {
	if before, _, found := strings.Cut(s, "/"); found {
		return before
	}
	return s
}

// parseLongPort handles the Docker Compose long syntax:
//
//	target: 8080
//	published: "8080"
//	protocol: tcp
//
// published may be a YAML string or integer depending on the Compose version.
func parseLongPort(node *yaml.Node) (Port, bool, error) {
	var lp struct {
		Target    uint16 `yaml:"target"`
		Published string `yaml:"published"`
	}
	if err := node.Decode(&lp); err != nil {
		return Port{}, false, fmt.Errorf("parsing long port syntax: %w", err)
	}
	if lp.Published == "" || lp.Target == 0 {
		return Port{}, false, nil
	}
	pub, err := strconv.ParseUint(lp.Published, 10, 16)
	if err != nil {
		return Port{}, false, fmt.Errorf("invalid published port %q: %w", lp.Published, err)
	}
	return Port{Host: uint16(pub), Container: lp.Target}, true, nil
}

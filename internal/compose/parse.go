package compose

import (
	"fmt"
	"os"
	"path/filepath"
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
	Image string   `yaml:"image"`
	Ports []string `yaml:"ports"`
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

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return Project{}, fmt.Errorf("parsing compose file: %w", err)
	}

	proj := Project{Name: cf.Name}
	for name, svc := range cf.Services {
		s := Service{
			Name:  name,
			Image: svc.Image,
		}
		for _, raw := range svc.Ports {
			p, ok, err := parsePort(raw)
			if err != nil {
				return Project{}, fmt.Errorf("service %s: %w", name, err)
			}
			if ok {
				s.Ports = append(s.Ports, p)
			}
		}
		proj.Services = append(proj.Services, s)
	}

	return proj, nil
}

// parsePort parses port strings in Docker Compose short syntax.
// Returns (port, true, nil) for mappings with a host port,
// or (Port{}, false, nil) for container-only ports (e.g. "8080") which tug skips.
func parsePort(raw string) (Port, bool, error) {
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
	host, err := strconv.ParseUint(hostStr, 10, 16)
	if err != nil {
		return Port{}, fmt.Errorf("invalid host port %q: %w", hostStr, err)
	}
	container, err := strconv.ParseUint(containerStr, 10, 16)
	if err != nil {
		return Port{}, fmt.Errorf("invalid container port %q: %w", containerStr, err)
	}
	return Port{Host: uint16(host), Container: uint16(container)}, nil
}

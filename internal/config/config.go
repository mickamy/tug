package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultComposeCommand = "docker compose"
	DefaultRuntimeCommand = "docker"
)

type Command struct {
	Compose string `yaml:"compose"`
	Runtime string `yaml:"runtime"`
}

// ServiceConfig holds per-service overrides.
type ServiceConfig struct {
	Kind string `yaml:"kind"` // "http" or "tcp"
}

var validKinds = map[string]struct{}{
	"":     {},
	"http": {},
	"tcp":  {},
}

func (s *ServiceConfig) UnmarshalYAML(unmarshal func(any) error) error {
	type raw ServiceConfig // avoid recursion
	var v raw
	if err := unmarshal(&v); err != nil {
		return err
	}
	if _, ok := validKinds[v.Kind]; !ok {
		return fmt.Errorf("invalid service kind %q (must be \"http\" or \"tcp\")", v.Kind)
	}
	*s = ServiceConfig(v)
	return nil
}

type Config struct {
	Command  Command                  `yaml:"command"`
	Services map[string]ServiceConfig `yaml:"services"`
}

func defaults() Config {
	return Config{
		Command: Command{
			Compose: DefaultComposeCommand,
			Runtime: DefaultRuntimeCommand,
		},
	}
}

// Load reads config from tug.yaml in projectDir (project-local) and globalPath (global),
// merging with project-local taking priority over global.
// Both files are optional; missing files are silently ignored.
func Load(projectDir, globalPath string) (Config, error) {
	cfg := defaults()

	global, err := loadFile(globalPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return cfg, err
	}
	if err == nil {
		merge(&cfg, global)
	}

	local, err := loadFile(filepath.Join(projectDir, "tug.yaml"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return cfg, err
	}
	if err == nil {
		merge(&cfg, local)
	}

	return cfg, nil
}

// LoadDefault loads config using the current directory and the standard
// global config path (~/.config/tug.yaml).
func LoadDefault() (Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return defaults(), err
	}

	var globalPath string
	if home, err := os.UserHomeDir(); err == nil {
		globalPath = filepath.Join(home, ".config", "tug.yaml")
	}

	return Load(cwd, globalPath)
}

func loadFile(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path) //nolint:gosec // path is from known config locations, not user input
	if err != nil {
		return cfg, fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}

func merge(base *Config, override Config) {
	if override.Command.Compose != "" {
		base.Command.Compose = override.Command.Compose
	}
	if override.Command.Runtime != "" {
		base.Command.Runtime = override.Command.Runtime
	}
	for name, svc := range override.Services {
		if base.Services == nil {
			base.Services = make(map[string]ServiceConfig)
		}
		base.Services[name] = svc
	}
}

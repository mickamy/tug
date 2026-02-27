package main

import (
	"fmt"
	"os"

	"github.com/mickamy/tug/internal/compose"
	"github.com/mickamy/tug/internal/config"
	"github.com/mickamy/tug/internal/exec"
)

const (
	tugDir       = ".tug"
	mergedPath   = ".tug/compose.yaml"
	overridePath = ".tug/override.yaml"
)

// env holds the common dependencies resolved once per command.
type env struct {
	cfg    config.Config
	runner exec.Runner
	// composeFile is the effective compose file to use.
	// Prefers the merged file (.tug/compose.yaml) if one exists
	// from a previous "tug up --override".
	composeFile string
	// sourceComposeFile is the original file before merging (user-specified or auto-detected).
	// handleUp uses this as the starting point for a fresh merge.
	sourceComposeFile string
}

func configure(flags globalFlags) (env, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		return env{}, fmt.Errorf("loading config: %w", err)
	}

	src := flags.composeFile
	if src == "" {
		f, findErr := compose.FindComposeFile(".")
		if findErr != nil {
			return env{}, fmt.Errorf("finding compose file: %w", findErr)
		}
		src = f
	}

	// Prefer the merged file if a previous "tug up --override" created one,
	// but only when the compose file was auto-detected (no explicit -f/--file).
	effective := src
	if flags.composeFile == "" {
		if _, err := os.Stat(mergedPath); err == nil {
			effective = mergedPath
		}
	}

	return env{
		cfg:               cfg,
		runner:            exec.New(cfg),
		composeFile:       effective,
		sourceComposeFile: src,
	}, nil
}

// runFileArgs builds -f flags for the effective base and tug override.
func runFileArgs(base string) []string {
	args := []string{"-f", base}
	if _, err := os.Stat(overridePath); err == nil {
		args = append(args, "-f", overridePath)
	}
	return args
}

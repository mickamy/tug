package main

import (
	"errors"
	"slices"
	"testing"
)

func TestParseArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		argv        []string
		composeFile string
		overrides   []string
		passthrough []string
		rest        []string
	}{
		{
			name:        "profile before subcommand is forwarded",
			argv:        []string{"--profile", "load", "up", "-d", "--wait"},
			passthrough: []string{"--profile", "load"},
			rest:        []string{"up", "-d", "--wait"},
		},
		{
			name:        "profile with inline value",
			argv:        []string{"--profile=load", "up"},
			passthrough: []string{"--profile=load"},
			rest:        []string{"up"},
		},
		{
			name:        "short project name flag consumes value",
			argv:        []string{"-p", "myproj", "down"},
			passthrough: []string{"-p", "myproj"},
			rest:        []string{"down"},
		},
		{
			name:        "unknown boolean flag forwarded without value",
			argv:        []string{"--dry-run", "up"},
			passthrough: []string{"--dry-run"},
			rest:        []string{"up"},
		},
		{
			name:        "tug file and override flags are consumed",
			argv:        []string{"-f", "compose.yaml", "--override", "a.yaml", "--override", "b.yaml", "up"},
			composeFile: "compose.yaml",
			overrides:   []string{"a.yaml", "b.yaml"},
			rest:        []string{"up"},
		},
		{
			name: "flags after subcommand are left untouched",
			argv: []string{"logs", "-f", "api"},
			rest: []string{"logs", "-f", "api"},
		},
		{
			name: "no args",
			argv: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			flags, showVersion, rest, err := parseArgs(tc.argv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if showVersion {
				t.Errorf("showVersion = true, want false")
			}
			if flags.composeFile != tc.composeFile {
				t.Errorf("composeFile = %q, want %q", flags.composeFile, tc.composeFile)
			}
			if !slices.Equal(flags.overrideFiles, tc.overrides) {
				t.Errorf("overrideFiles = %v, want %v", flags.overrideFiles, tc.overrides)
			}
			if !slices.Equal(flags.passthrough, tc.passthrough) {
				t.Errorf("passthrough = %v, want %v", flags.passthrough, tc.passthrough)
			}
			if !slices.Equal(rest, tc.rest) {
				t.Errorf("rest = %v, want %v", rest, tc.rest)
			}
		})
	}
}

func TestParseArgs_Version(t *testing.T) {
	t.Parallel()

	_, showVersion, _, err := parseArgs([]string{"--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !showVersion {
		t.Error("showVersion = false, want true")
	}
}

func TestParseArgs_Help(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseArgs([]string{"-h"})
	if !errors.Is(err, errHelp) {
		t.Errorf("err = %v, want errHelp", err)
	}
}

func TestParseArgs_MissingValue(t *testing.T) {
	t.Parallel()

	_, _, _, err := parseArgs([]string{"-f"})
	if err == nil {
		t.Fatal("expected error for missing flag value")
	}
}

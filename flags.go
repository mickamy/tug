package main

import (
	"errors"
	"fmt"
	"strings"
)

// globalFlags holds tug's own global flags plus any unrecognized global flags
// that are forwarded verbatim to the underlying compose command.
type globalFlags struct {
	composeFile   string
	overrideFiles []string
	// passthrough holds global flags tug doesn't define (e.g. --profile,
	// --project-name), forwarded to compose ahead of the subcommand.
	passthrough []string
}

// errHelp signals that usage was requested via -h/--help.
var errHelp = errors.New("help requested")

// composeGlobalValueFlags lists compose top-level flags that consume a separate
// value. tug forwards unrecognized global flags to compose; knowing which take
// a value lets it tell a flag's value apart from the subcommand.
var composeGlobalValueFlags = map[string]bool{
	"profile":           true,
	"p":                 true,
	"project-name":      true,
	"project-directory": true,
	"env-file":          true,
	"progress":          true,
	"ansi":              true,
	"parallel":          true,
}

// parseArgs splits argv into tug's global flags, a version request, and the
// remaining command (subcommand plus its args). Parsing stops at the first
// non-flag token, so flags after the subcommand are left for compose.
func parseArgs(argv []string) (globalFlags, bool, []string, error) {
	var (
		flags       globalFlags
		showVersion bool
	)
	i := 0
	for i < len(argv) {
		arg := argv[i]
		if arg == "--" {
			return flags, showVersion, argv[i+1:], nil
		}
		if len(arg) < 2 || arg[0] != '-' {
			return flags, showVersion, argv[i:], nil
		}

		name, value, hasValue := splitFlag(arg)
		switch name {
		case "f", "file":
			v, next, err := takeValue(argv, i, value, hasValue)
			if err != nil {
				return flags, showVersion, nil, err
			}
			flags.composeFile = v
			i = next
		case "override":
			v, next, err := takeValue(argv, i, value, hasValue)
			if err != nil {
				return flags, showVersion, nil, err
			}
			flags.overrideFiles = append(flags.overrideFiles, v)
			i = next
		case "v", "version":
			showVersion = true
			i++
		case "h", "help":
			return flags, showVersion, nil, errHelp
		default:
			if hasValue || !composeGlobalValueFlags[name] {
				flags.passthrough = append(flags.passthrough, arg)
				i++
				continue
			}
			v, next, err := takeValue(argv, i, value, hasValue)
			if err != nil {
				return flags, showVersion, nil, err
			}
			flags.passthrough = append(flags.passthrough, arg, v)
			i = next
		}
	}
	return flags, showVersion, nil, nil
}

// splitFlag strips leading dashes and separates an inline "=value".
func splitFlag(arg string) (name, value string, hasValue bool) {
	s := strings.TrimLeft(arg, "-")
	if name, value, found := strings.Cut(s, "="); found {
		return name, value, true
	}
	return s, "", false
}

// takeValue resolves a flag's value, either inline ("--flag=v") or from the
// next token ("--flag v").
func takeValue(argv []string, i int, inline string, hasInline bool) (string, int, error) {
	if hasInline {
		return inline, i + 1, nil
	}
	if i+1 >= len(argv) {
		return "", i, fmt.Errorf("flag needs an argument: %s", argv[i])
	}
	return argv[i+1], i + 2, nil
}

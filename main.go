package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	var (
		flags       globalFlags
		showVersion bool
	)

	fs := flag.NewFlagSet("tug", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = printUsage
	fs.StringVar(&flags.composeFile, "f", "", "")
	fs.StringVar(&flags.composeFile, "file", "", "")
	fs.Var(&flags.overrideFiles, "override", "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&showVersion, "v", false, "")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if showVersion {
		fmt.Println("tug", version)
		return 0
	}

	rest := fs.Args()
	if len(rest) == 0 {
		printUsage()
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch rest[0] {
	case "up":
		err = handleUp(ctx, flags, rest[1:])
	case "down":
		err = handleDown(ctx, flags, rest[1:])
	case "ps":
		err = handlePs(ctx, flags, rest[1:])
	case "prune":
		err = handlePrune(ctx)
	default:
		err = passthrough(ctx, flags, rest)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "tug: %v\n", err)
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `tug - Docker Compose with auto-routing

Usage:
  tug [-f <file>] [--override <file>]... <command> [args...]

Commands:
  up       Start services with Traefik routing and deterministic ports
  down     Stop services
  ps       Show services with URLs and port mappings (--json for JSON output)
  prune    Stop and remove the Traefik container and tug network

Any other command is forwarded to docker compose as-is.
  e.g.  tug logs -f api
        tug exec db psql

Flags:
  -f, --file       Specify compose file (default: auto-detect)
  --override       Layer an additional override file (repeatable)
  --version, -v    Print version
  -h, --help       Show this help
`)
}

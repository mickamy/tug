package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/mickamy/tug/internal/compose"
	"github.com/mickamy/tug/internal/config"
	"github.com/mickamy/tug/internal/exec"
	"github.com/mickamy/tug/internal/merge"
	"github.com/mickamy/tug/internal/override"
	"github.com/mickamy/tug/internal/traefik"
)

func handleUp(ctx context.Context, flags globalFlags, args []string) error {
	e, err := configure(flags)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(tugDir, 0o750); err != nil {
		return fmt.Errorf("creating %s directory: %w", tugDir, err)
	}

	// Merge base + user overrides, or clean up stale merged file.
	base := e.sourceComposeFile
	if len(flags.overrideFiles) > 0 {
		files := make([]string, 0, 1+len(flags.overrideFiles))
		files = append(files, e.sourceComposeFile)
		files = append(files, flags.overrideFiles...)
		merged, err := merge.Compose(ctx, e.runner, files...)
		if err != nil {
			return fmt.Errorf("preparing merged compose: %w", err)
		}
		if err := os.WriteFile(mergedPath, merged, 0o600); err != nil {
			return fmt.Errorf("writing merged compose: %w", err)
		}
		base = mergedPath
	} else {
		_ = os.Remove(mergedPath)
	}

	proj, err := compose.Parse(base)
	if err != nil {
		return fmt.Errorf("parsing compose file: %w", err)
	}

	classified, err := override.Classify(proj, e.cfg)
	if err != nil {
		return fmt.Errorf("classifying services: %w", err)
	}

	data, err := override.Generate(proj, classified)
	if err != nil {
		return fmt.Errorf("generating override: %w", err)
	}

	if err := os.WriteFile(overridePath, data, 0o600); err != nil {
		return fmt.Errorf("writing override: %w", err)
	}

	if err := traefik.EnsureRunning(ctx, e.runner); err != nil {
		return fmt.Errorf("ensuring traefik: %w", err)
	}

	composeArgs := make([]string, 0, 5+len(args))
	composeArgs = append(composeArgs, "-f", base, "-f", overridePath, "up")
	composeArgs = append(composeArgs, args...)
	if err := e.runner.Compose(ctx, composeArgs...); err != nil {
		return fmt.Errorf("compose up: %w", err)
	}
	return nil
}

func handleDown(ctx context.Context, flags globalFlags, args []string) error {
	e, err := configure(flags)
	if err != nil {
		return err
	}

	composeArgs := runFileArgs(e.composeFile)
	composeArgs = append(composeArgs, "down")
	composeArgs = append(composeArgs, args...)
	if err := e.runner.Compose(ctx, composeArgs...); err != nil {
		return fmt.Errorf("compose down: %w", err)
	}

	// Clean up generated files so stale state doesn't leak into future runs.
	_ = os.Remove(mergedPath)
	_ = os.Remove(overridePath)

	return nil
}

func handlePs(ctx context.Context, flags globalFlags, args []string) error {
	if !tugActive() {
		fmt.Fprintln(os.Stderr, "tug: not active (run \"tug up\" first)")
		return nil
	}

	e, err := configure(flags)
	if err != nil {
		return err
	}

	proj, err := compose.Parse(e.composeFile)
	if err != nil {
		return fmt.Errorf("parsing compose file: %w", err)
	}

	classified, err := override.Classify(proj, e.cfg)
	if err != nil {
		return fmt.Errorf("classifying services: %w", err)
	}

	statuses := containerStatuses(ctx, e.runner, e.composeFile)
	rows := buildPsRows(proj, classified, statuses)

	if slices.Contains(args, "--json") {
		return writePsJSON(rows)
	}
	return writePsTable(rows)
}

func passthrough(ctx context.Context, flags globalFlags, args []string) error {
	e, err := configure(flags)
	if err != nil {
		return err
	}

	composeArgs := runFileArgs(e.composeFile)
	composeArgs = append(composeArgs, args...)

	cmd := args[0]
	if err := e.runner.Compose(ctx, composeArgs...); err != nil {
		return fmt.Errorf("compose %s: %w", cmd, err)
	}
	return nil
}

// psRow represents a single row in the tug ps output.
type psRow struct {
	Service string `json:"service"`
	Type    string `json:"type"`
	URLPort string `json:"endpoint"`
	Status  string `json:"status"`
}

func buildPsRows(
	proj compose.Project,
	classified []override.ClassifiedService,
	statuses map[string]string,
) []psRow {
	var rows []psRow
	for _, cs := range classified {
		status := statuses[cs.Name]
		for _, cp := range cs.ClassifiedPorts {
			var urlPort string
			switch cp.Kind {
			case override.KindHTTP:
				urlPort = fmt.Sprintf(
					"http://%s.%s.localhost", cs.Name, proj.Name,
				)
			case override.KindTCP:
				if cp.HostPort > 0 {
					urlPort = fmt.Sprintf(
						"localhost:%d → %d", cp.HostPort, cp.ContainerPort,
					)
				}
			}
			rows = append(rows, psRow{
				Service: cs.Name,
				Type:    cp.Kind.String(),
				URLPort: urlPort,
				Status:  status,
			})
		}
	}
	return rows
}

func writePsTable(rows []psRow) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tTYPE\tURL/PORT\tSTATUS")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			r.Service, r.Type, r.URLPort, r.Status,
		)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("flushing output: %w", err)
	}
	return nil
}

func writePsJSON(rows []psRow) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

func handlePrune(ctx context.Context) error {
	cfg, err := config.LoadDefault()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := traefik.Stop(ctx, exec.New(cfg)); err != nil {
		return fmt.Errorf("stopping traefik: %w", err)
	}
	// Clean up generated files so stale state doesn't leak into future runs.
	_ = os.Remove(mergedPath)
	_ = os.Remove(overridePath)
	_ = os.Remove(tugDir)
	return nil
}

// psEntry holds the fields we need from docker compose ps --format json.
type psEntry struct {
	Service string `json:"Service"` //nolint:tagliatelle // Docker Compose API uses PascalCase
	State   string `json:"State"`   //nolint:tagliatelle // Docker Compose API uses PascalCase
}

// containerStatuses queries docker compose ps and returns a service → state map.
// Returns nil on error (e.g. no containers running).
func containerStatuses(ctx context.Context, runner exec.Runner, base string) map[string]string {
	args := runFileArgs(base)
	args = append(args, "ps", "--format", "json")
	out, err := runner.ComposeOutput(ctx, args...)
	if err != nil {
		return nil
	}

	var entries []psEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		// Fall back to NDJSON (one object per line) for older Compose versions.
		for line := range bytes.SplitSeq(out, []byte("\n")) {
			var e psEntry
			if json.Unmarshal(line, &e) == nil && e.Service != "" {
				entries = append(entries, e)
			}
		}
	}

	statuses := make(map[string]string, len(entries))
	for _, e := range entries {
		statuses[e.Service] = e.State
	}
	return statuses
}

package merge

import (
	"context"
	"fmt"

	"github.com/mickamy/tug/internal/exec"
)

// Compose runs `docker compose config` over the given files and returns
// the fully resolved, merged YAML. The files are applied in order, so
// later files override earlier ones.
func Compose(ctx context.Context, runner exec.Runner, files ...string) ([]byte, error) {
	args := make([]string, 0, 2*len(files)+1)
	for _, f := range files {
		args = append(args, "-f", f)
	}
	args = append(args, "config")

	out, err := runner.ComposeOutput(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("merging compose files: %w", err)
	}
	return out, nil
}

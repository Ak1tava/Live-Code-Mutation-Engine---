package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const maxRunnerOutput = 4 << 20

// Local runs the runner binary directly on the host.
type Local struct {
	runnerBin string
}

// NewLocal returns a Local that executes runnerBin.
func NewLocal(runnerBin string) *Local { return &Local{runnerBin: runnerBin} }

var _ Runner = (*Local)(nil)

// Env is the environment variables forced on every runner process.
var Env = []string{"GOTOOLCHAIN=local", "GOPROXY=off", "GOFLAGS=-mod=readonly"}

// Run executes the batch and returns one result per id, in order.
func (l *Local) Run(ctx context.Context, b Batch) ([]VariantResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, l.runnerBin, "-dir", b.Dir, "-ids", strings.Join(b.IDs, ","))
	cmd.Env = SandboxEnv()
	SetProcessGroup(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	res, perr := ParseResults(stdout, maxRunnerOutput)
	if perr != nil {
		cancel()
	}
	werr := cmd.Wait()
	if perr != nil {
		return nil, perr
	}
	if werr != nil {
		return nil, fmt.Errorf("runner: %w: %s", werr, strings.TrimSpace(stderr.String()))
	}
	if len(res) != len(b.IDs) {
		return nil, fmt.Errorf("runner returned %d results for %d variants", len(res), len(b.IDs))
	}
	return res, nil
}

// GoVersion returns `go env GOVERSION`.
func (l *Local) GoVersion(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "go", "env", "GOVERSION").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxStderr  = 4096
	killWindow = 10 * time.Second
	waitDelay  = 2 * time.Second
)

// Docker runs batches in a throwaway container without network or privileges.
type Docker struct {
	image   string
	timeout time.Duration
}

// NewDocker returns a Docker runner for image with a per-Run timeout.
func NewDocker(image string, timeout time.Duration) *Docker {
	return &Docker{image: image, timeout: timeout}
}

var _ Runner = (*Docker)(nil)

// runArgs builds the `docker run` argument list. batchDir must be absolute.
func (d *Docker) runArgs(name, batchDir string, ids []string) []string {
	return []string{
		"run", "--rm",
		"--network", "none",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"-u", "65534:65534",
		"--cpus", "2",
		"--memory", "1g",
		"--pids-limit", "512",
		"--name", name,
		"--mount", "type=bind,src=" + batchDir + ",dst=/work,readonly",
		d.image,
		"lcme-runner", "-dir", "/work", "-ids", strings.Join(ids, ","),
	}
}

func containerName() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "lcme-" + hex.EncodeToString(b[:]), nil
}

// cappedBuffer keeps only the first max bytes written to it.
type cappedBuffer struct {
	buf bytes.Buffer
	max int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := c.max - c.buf.Len(); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		c.buf.Write(p[:room])
	}
	return len(p), nil
}

// Run executes the batch and returns one result per id, in order.
func (d *Docker) Run(ctx context.Context, b Batch) ([]VariantResult, error) {
	dir, err := filepath.Abs(b.Dir)
	if err != nil {
		return nil, err
	}
	name, err := containerName()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", d.runArgs(name, dir, b.IDs)...)
	cmd.WaitDelay = waitDelay
	stderr := &cappedBuffer{max: maxStderr}
	cmd.Stderr = stderr
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
	cerr := ctx.Err()
	if cerr != nil {
		// Killing the docker client does not stop the container; the original
		// context is dead, so use a fresh one.
		kctx, kcancel := context.WithTimeout(context.Background(), killWindow)
		defer kcancel()
		_ = exec.CommandContext(kctx, "docker", "kill", name).Run()
	}
	if perr != nil {
		return nil, perr
	}
	if werr != nil {
		return nil, fmt.Errorf("docker run: %w: %s", werr, strings.TrimSpace(stderr.buf.String()))
	}
	if cerr != nil {
		return nil, fmt.Errorf("docker run: %w", cerr)
	}
	if len(res) != len(b.IDs) {
		return nil, fmt.Errorf("runner returned %d results for %d variants: %s",
			len(res), len(b.IDs), strings.TrimSpace(stderr.buf.String()))
	}
	return res, nil
}

// GoVersion returns `go env GOVERSION` of the image toolchain.
func (d *Docker) GoVersion(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", "run", "--rm", d.image, "go", "env", "GOVERSION").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

package project

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const importImage = "lcme-importer:latest"

// Bound aggregate host resources too: each import owns a proxy and archive reader.
var importSlots = make(chan struct{}, 2)

// GitClone clones public GitHub repositories in a resource-limited, networkless
// container. Its only outbound channel is the scoped HTTPS proxy in proxy.go.
// There is deliberately no fallback to host git when Docker/image is unavailable.
func GitClone(ctx context.Context, url, dst string) error {
	return gitClone(ctx, url, dst, DefaultLimits)
}

func gitClone(ctx context.Context, url, dst string, lim Limits) error {
	return gitCloneVia(ctx, url, dst, lim, publicGitHubDial)
}

// dial is injectable only inside this package for hermetic transport tests.
func gitCloneVia(ctx context.Context, url, dst string, lim Limits, dial importDial) error {
	if err := checkURL(url); err != nil {
		return fmt.Errorf("%w: %w", ErrClone, err)
	}
	ctx, cancel := context.WithTimeout(ctx, min(lim.CloneTimeout, 60*time.Second))
	defer cancel()
	select {
	case importSlots <- struct{}{}:
		defer func() { <-importSlots }()
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrClone, ctx.Err())
	}
	socket, stop, err := startImportProxy(ctx, dial)
	if err != nil {
		return fmt.Errorf("%w: proxy: %w", ErrClone, err)
	}
	defer stop()
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Errorf("%w: %w", ErrClone, err)
	}
	name := "lcme-import-" + hex.EncodeToString(random[:])
	cmd := exec.CommandContext(ctx, "docker", importArgs(name, socket, url)...)
	cmd.WaitDelay = 2 * time.Second
	stderr := &importStderr{}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrClone, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %w", ErrClone, err)
	}
	// Killing docker's client alone does not kill the container. Also covers a
	// cancelled docker run that creates its container just before client exit.
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	}()
	extractErr := extractImport(stdout, dst, lim)
	if extractErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	if errors.Is(extractErr, ErrTooLarge) {
		return ErrTooLarge
	}
	if extractErr != nil || waitErr != nil {
		return fmt.Errorf("%w: %v: %s", ErrClone, errors.Join(extractErr, waitErr), strings.TrimSpace(stderr.String()))
	}
	return nil
}

func importArgs(name, socket, url string) []string {
	return []string{
		"run", "--rm", "--pull", "never", "--name", name,
		"--network", "none", "--read-only", "--cap-drop", "ALL",
		"--security-opt", "no-new-privileges", "--user", "65534:65534",
		"--cpus", "1", "--memory", "256m", "--memory-swap", "256m",
		"--pids-limit", "64", "--ulimit", "nofile=128:128", "--log-driver", "none",
		"--tmpfs", "/work:rw,noexec,nosuid,nodev,size=64m,nr_inodes=4096,mode=1777",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=1m,nr_inodes=64,mode=1777",
		"--mount", "type=bind,src=" + socket + ",dst=/proxy.sock,readonly",
		importImage, url,
	}
}

// Docker stderr may contain arbitrary remote error messages; retain only 4 KiB.
type importStderr struct{ buf bytes.Buffer }

func (b *importStderr) Len() int       { return b.buf.Len() }
func (b *importStderr) String() string { return b.buf.String() }

func (b *importStderr) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 4096 - b.Len(); remaining > 0 {
		_, _ = b.buf.Write(p[:min(remaining, n)])
	}
	return n, nil
}

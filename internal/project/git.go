package project

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// GitClone делает неглубокий клон url в dst. Интерактивные запросы пароля отключены.
func GitClone(ctx context.Context, url, dst string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--no-tags", "--single-branch", "--", url, dst)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	// Own process group, so on timeout git-remote-https and other children die too.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	// Force-close the stderr pipe if a stray child still holds it open.
	cmd.WaitDelay = 3 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", ErrClone, lastLine(stderr.String(), err))
	}
	return nil
}

func lastLine(s string, fallback error) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if l := strings.TrimSpace(lines[len(lines)-1]); l != "" {
		return l
	}
	return fallback.Error()
}

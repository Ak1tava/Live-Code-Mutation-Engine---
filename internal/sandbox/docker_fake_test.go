package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A fake `docker` on PATH lets Run's error handling be tested without Docker.
const fakeDockerScript = `#!/bin/sh
echo "$*" >> "$FAKE_LOG"
[ "$1" = kill ] && exit 0
case "$FAKE_MODE" in
ok) printf '{"id":"v1","tests":[]}\n'; exit 0;;
hang) exec sleep 100;;
full_then_hang) printf '{"id":"v1","tests":[]}\n'; exec sleep 100;;
partial_hang) printf '{"id":"v1","tests":['; exec sleep 100;;
partial_exit) printf '{"id":"v1","te'; echo "runner boom" >&2; exit 1;;
wrong_id) printf '{"id":"zzz","tests":[]}\n'; exit 0;;
version_fail) echo "Cannot connect to the Docker daemon" >&2; exit 1;;
esac
`

func fakeDocker(t *testing.T, mode string) (log string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(fakeDockerScript), 0o755); err != nil {
		t.Fatal(err)
	}
	log = filepath.Join(dir, "log")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_LOG", log)
	t.Setenv("FAKE_MODE", mode)
	return log
}

func fakeRun(t *testing.T, mode string) ([]VariantResult, string, error) {
	t.Helper()
	log := fakeDocker(t, mode)
	res, err := NewDocker("img:1", time.Second).Run(context.Background(), Batch{Dir: "rel/batch", IDs: []string{"v1"}})
	data, _ := os.ReadFile(log)
	return res, string(data), err
}

func TestFakeDockerOK(t *testing.T) {
	res, log, err := fakeRun(t, "ok")
	if err != nil || len(res) != 1 || res[0].ID != "v1" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	abs, _ := filepath.Abs("rel/batch")
	if !strings.Contains(log, "src="+abs+",") {
		t.Errorf("src not absolute: %s", log)
	}
}

func TestFakeDockerTimeout(t *testing.T) {
	for _, mode := range []string{"hang", "partial_hang", "full_then_hang"} {
		t.Run(mode, func(t *testing.T) {
			start := time.Now()
			_, log, err := fakeRun(t, mode)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("err = %v, want DeadlineExceeded", err)
			}
			if time.Since(start) > 6*time.Second {
				t.Errorf("took %v", time.Since(start))
			}
			if !strings.Contains(log, "kill lcme-") {
				t.Errorf("docker kill not called: %s", log)
			}
		})
	}
}

func TestFakeDockerCallerDeadline(t *testing.T) {
	fakeDocker(t, "hang")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := NewDocker("img:1", time.Minute).Run(ctx, Batch{Dir: t.TempDir(), IDs: []string{"v1"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}

func TestFakeDockerCallerCancel(t *testing.T) {
	fakeDocker(t, "hang")
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(300*time.Millisecond, cancel)
	_, err := NewDocker("img:1", time.Minute).Run(ctx, Batch{Dir: t.TempDir(), IDs: []string{"v1"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestFakeDockerKeepsStderr(t *testing.T) {
	_, _, err := fakeRun(t, "partial_exit")
	if err == nil || !strings.Contains(err.Error(), "runner boom") {
		t.Fatalf("err = %v", err)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		t.Errorf("parse error misreported as context error: %v", err)
	}
}

func TestFakeDockerWrongID(t *testing.T) {
	if _, _, err := fakeRun(t, "wrong_id"); err == nil || !strings.Contains(err.Error(), "zzz") {
		t.Fatalf("err = %v", err)
	}
}

func TestFakeDockerGoVersionStderr(t *testing.T) {
	fakeDocker(t, "version_fail")
	_, err := NewDocker("img:1", time.Second).GoVersion(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Docker daemon") {
		t.Fatalf("err = %v", err)
	}
}

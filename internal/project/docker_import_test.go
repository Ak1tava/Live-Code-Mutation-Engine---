package project

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func requireImportDocker(t *testing.T) {
	t.Helper()
	if os.Getenv("LCME_DOCKER") != "1" {
		t.Skip("set LCME_DOCKER=1 for importer container integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "docker", "image", "inspect", importImage).Run(); err != nil {
		t.Fatalf("build sandbox/import.Dockerfile as %s first: %v", importImage, err)
	}
}

func TestDockerImportIsolationAndQuotas(t *testing.T) {
	requireImportDocker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	socket, stop, err := startImportProxy(ctx, func(context.Context) (net.Conn, error) { return nil, errors.New("offline test") })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	name := fmt.Sprintf("lcme-import-isolation-%d", time.Now().UnixNano())
	args := importArgs(name, socket, goodURL)
	args = args[:len(args)-2]
	args = append(args, "--entrypoint", "/bin/sh", importImage, "-ec", `
test "$(id -u)" = 65534
test "$(ls /sys/class/net)" = lo
test -S /proxy.sock
test "$(cat /sys/fs/cgroup/memory.max)" = 268435456
test "$(cat /sys/fs/cgroup/memory.swap.max)" = 0
test "$(cat /sys/fs/cgroup/pids.max)" = 64
test "$(cat /sys/fs/cgroup/cpu.max)" = '100000 100000'
! touch /root-is-read-only 2>/dev/null
test -z "${GIT_CONFIG_COUNT:-}"
test "$(git config --global --list | wc -l)" = 0
! dd if=/dev/zero of=/work/large bs=1048576 count=65 2>/dev/null
rm /work/large
i=0
while mkdir "/work/inode-$i" 2>/dev/null; do i=$((i + 1)); done
test "$i" -lt 4096
test "$i" -gt 3900
printf 'isolation and quotas ok\n'
`)
	// These host settings must never be inherited by git in the container.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "credential.helper")
	t.Setenv("GIT_CONFIG_VALUE_0", "!false")
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("container isolation: %v: %s", err, out)
	}
	if string(out) != "isolation and quotas ok\n" {
		t.Fatalf("unexpected output %q", out)
	}
}

func TestDockerImportRejectsRedirect(t *testing.T) {
	requireImportDocker(t)
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Location", "https://github.com/example/redirected.git")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	socket, stop, err := startImportProxy(ctx, func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	args := importArgs(fmt.Sprintf("lcme-import-redirect-%d", time.Now().UnixNano()), socket, goodURL)
	// Fixture-only: trust the local TLS server. Production never sets this env.
	args = append(args[:len(args)-2], "--env", "GIT_SSL_NO_VERIFY=true", importImage, goodURL)
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err == nil {
		t.Fatal("redirect accepted")
	}
	if requests.Load() != 1 || !strings.Contains(string(out), "302") {
		t.Fatalf("redirect requests=%d, output=%s, err=%v", requests.Load(), out, err)
	}
}

func TestDockerImportLiveGitHub(t *testing.T) {
	requireImportDocker(t)
	if os.Getenv("LCME_IMPORT_LIVE") != "1" {
		t.Skip("set LCME_IMPORT_LIVE=1 for the public GitHub smoke")
	}
	dst := filepath.Join(t.TempDir(), "repo")
	// Small public Go module; this exercises live DNS, TLS, CONNECT and checkout.
	if err := GitClone(context.Background(), "https://github.com/deckarep/golang-set.git", dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git exported: %v", err)
	}
}

func TestDockerImportCancellationRemovesContainerAndSocket(t *testing.T) {
	requireImportDocker(t)
	realDocker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal(err)
	}
	spyDir := t.TempDir()
	nameFile := filepath.Join(spyDir, "container-name")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	// Record this call's generated name without depending on global Docker state.
	script := "#!/bin/sh\nif [ \"$1\" = run ]; then\n  previous=\n  for arg do\n    if [ \"$previous\" = --name ]; then printf '%s' \"$arg\" > " + quote(nameFile) + "; fi\n    previous=\"$arg\"\n  done\nfi\nexec " + quote(realDocker) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(spyDir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", spyDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	started := make(chan struct{}, 1)
	done := make(chan error, 1)
	dst := t.TempDir()
	go func() {
		done <- gitCloneVia(ctx, goodURL, dst, DefaultLimits, func(ctx context.Context) (net.Conn, error) {
			a, b := net.Pipe()
			go func() {
				defer b.Close()
				stop := context.AfterFunc(ctx, func() { _ = b.Close() })
				defer stop()
				_, _ = io.Copy(io.Discard, b) // Receive TLS hello, never answer it.
			}()
			select {
			case started <- struct{}{}:
			default:
			}
			return a, nil
		})
	}()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("clone exited before CONNECT: %v", err)
	case <-ctx.Done():
		t.Fatal("clone never reached CONNECT")
	}
	nameBytes, err := os.ReadFile(nameFile)
	if err != nil {
		t.Fatal(err)
	}
	name := string(nameBytes)
	socketBytes, err := exec.CommandContext(ctx, realDocker, "inspect", "--format", `{{range .Mounts}}{{if eq .Destination "/proxy.sock"}}{{.Source}}{{end}}{{end}}`, name).Output()
	if err != nil {
		t.Fatal(err)
	}
	socket := strings.TrimSpace(string(socketBytes))
	if socket == "" {
		t.Fatal("proxy socket mount missing")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, ErrClone) {
			t.Fatalf("cancel error=%v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("cancel did not terminate clone promptly")
	}
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer checkCancel()
	if err := exec.CommandContext(checkCtx, realDocker, "inspect", name).Run(); err == nil {
		t.Fatal("cancelled container still exists")
	}
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Fatalf("cancelled proxy socket still exists: %v", err)
	}
}

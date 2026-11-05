package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lcme/internal/mutate"
)

const testImage = "lcme-runner:latest"

func dockerRunner(t *testing.T) *Docker {
	t.Helper()
	if os.Getenv("LCME_DOCKER") != "1" {
		t.Skip("set LCME_DOCKER=1 to run Docker tests")
	}
	return NewDocker(testImage, 180*time.Second)
}

// dockerBatch builds a one-variant batch (WriteVariant makes Dir traversable).
func dockerBatch(t *testing.T, files map[string][]byte, ms ...mutate.Mutation) Batch {
	t.Helper()
	return variant(t, files, ms...)
}

func dockerRunOne(t *testing.T, b Batch) VariantResult {
	t.Helper()
	res, err := dockerRunner(t).Run(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1", len(res))
	}
	return res[0]
}

func TestDockerRunArgs(t *testing.T) {
	d := NewDocker("img:1", time.Minute)
	args := d.runArgs("lcme-0123456789abcdef", "/abs/batch", []string{"a", "b"})
	line := " " + strings.Join(args, " ") + " "
	for _, want := range []string{
		" --rm ",
		" --network none ",
		" --cap-drop ALL ",
		" --security-opt no-new-privileges ",
		" -u 65534:65534 ",
		" --cpus 2 ",
		" --memory 1g ",
		" --memory-swap 1g ",
		" --read-only ",
		" --log-driver none ",
		" --tmpfs /tmp:rw,exec,nosuid,nodev,size=268435456,mode=1777 ",
		" --pids-limit 512 ",
		" --name lcme-0123456789abcdef ",
		" --mount type=bind,src=/abs/batch,dst=/work,readonly ",
		" img:1 lcme-runner -dir /work -ids a,b ",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in %q", want, line)
		}
	}
	if args[0] != "run" {
		t.Errorf("args[0] = %q", args[0])
	}
	if i := slices.Index(args, "img:1"); i < 0 || slices.Index(args, "--rm") > i {
		t.Errorf("flags must precede image: %v", args)
	}
}

func TestContainerName(t *testing.T) {
	a, err := containerName()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := containerName()
	if len(a) != len("lcme-")+16 || !strings.HasPrefix(a, "lcme-") || a == b {
		t.Errorf("names %q %q", a, b)
	}
}

func TestCappedBuffer(t *testing.T) {
	c := &cappedBuffer{max: 5}
	if n, err := c.Write([]byte("abcdefgh")); n != 8 || err != nil {
		t.Fatalf("n=%d err=%v", n, err)
	}
	c.Write([]byte("zzz"))
	if got := c.buf.String(); got != "abcde" {
		t.Errorf("got %q", got)
	}
}

func TestDockerTestdataAreValidModules(t *testing.T) {
	for _, name := range []string{"net", "whoami"} {
		cmd := exec.Command("go", "vet", "./...")
		cmd.Dir = filepath.Join("testdata", name)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%s: go vet: %v\n%s", name, err, out)
		}
	}
}

func TestDockerBaselineAllPass(t *testing.T) {
	dockerRunner(t)
	start := time.Now()
	r := dockerRunOne(t, dockerBatch(t, textkit(t)))
	if elapsed := time.Since(start); elapsed >= 5*time.Second {
		t.Errorf("warm-cache baseline took %v, want < 5s", elapsed)
	} else {
		t.Logf("warm-cache baseline: %v", elapsed)
	}
	if r.BuildError != "" {
		t.Fatalf("BuildError: %s", r.BuildError)
	}
	if len(r.Tests) != 5 {
		t.Fatalf("tests = %d, want 5: %+v", len(r.Tests), r.Tests)
	}
	for _, tr := range r.Tests {
		if tr.Status != "pass" {
			t.Errorf("%s: %s\n%s", tr.Key(), tr.Status, tr.Output)
		}
	}
}

func TestDockerStorageAndMemoryLimits(t *testing.T) {
	dockerRunner(t)
	files := tinyProject(`import (
 "errors"
 "os"
 "path/filepath"
 "strings"
 "syscall"
 "testing"
)
func TestLimits(t *testing.T) {
 // A single byte proves rootfs immutability without filling any filesystem.
 if err := os.WriteFile("/opt/gocache/lcme-readonly-probe", []byte("x"), 0600); !errors.Is(err, syscall.EROFS) {
  t.Errorf("rootfs write = %v, want EROFS", err)
 }
 var stat syscall.Statfs_t
 if err := syscall.Statfs("/tmp", &stat); err != nil { t.Fatal(err) }
 if stat.Type != 0x01021994 || stat.Blocks*uint64(stat.Bsize) != 256<<20 {
  t.Errorf("/tmp type=%x size=%d, want tmpfs 256 MiB", stat.Type, stat.Blocks*uint64(stat.Bsize))
 }
 if cache := os.Getenv("GOCACHE"); cache != "/tmp/gocache" { t.Errorf("GOCACHE = %q", cache) }
 if err := os.WriteFile(filepath.Join(os.TempDir(), "lcme-writable-probe"), []byte("x"), 0600); err != nil {
  t.Errorf("bounded /tmp must remain writable: %v", err)
 }
 // Support both cgroup versions; actual kernel values establish enforcement.
 read := func(path string) string {
  b, err := os.ReadFile(path); if err != nil { t.Fatal(err) }; return strings.TrimSpace(string(b))
 }
 if _, err := os.Stat("/sys/fs/cgroup/memory.max"); err == nil {
  if got := read("/sys/fs/cgroup/memory.max"); got != "1073741824" { t.Errorf("memory.max=%s", got) }
  if got := read("/sys/fs/cgroup/memory.swap.max"); got != "0" { t.Errorf("memory.swap.max=%s", got) }
 } else {
  base := "/sys/fs/cgroup/memory/"
  if got := read(base+"memory.limit_in_bytes"); got != "1073741824" { t.Errorf("memory limit=%s", got) }
  if got := read(base+"memory.memsw.limit_in_bytes"); got != "1073741824" { t.Errorf("memory+swap limit=%s", got) }
 }
}
`, "")
	r := dockerRunOne(t, dockerBatch(t, files))
	if r.BuildError != "" || len(r.Tests) != 1 || r.Tests[0].Status != StatusPass {
		t.Fatalf("storage/memory limits: %+v", r)
	}
}

func TestDockerLogDriverDisabled(t *testing.T) {
	d := dockerRunner(t)
	name, err := containerName()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if out, err := exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).CombinedOutput(); err != nil {
			t.Errorf("remove inspection container: %v: %s", err, out)
		}
	})
	args := d.runArgs(name, t.TempDir(), []string{"unused"})
	args = append(args[:slices.Index(args, testImage)+1], "sleep", "30")
	args = append([]string{"run", "--detach"}, args[1:]...)
	if out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput(); err != nil {
		t.Fatalf("start inspection container: %v: %s", err, out)
	}
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.HostConfig.LogConfig.Type}}", name).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "none" {
		t.Fatalf("Docker log driver = %q, err=%v; want none", out, err)
	}
}

func TestDockerPanicIsolated(t *testing.T) {
	dockerRunner(t)
	files := textkit(t)
	src := map[string][]byte{}
	for n, b := range files {
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			src[n] = b
		}
	}
	ms, err := mutate.Generate(src)
	if err != nil {
		t.Fatal(err)
	}
	var pick *mutate.Mutation
	for i := range ms {
		if ms[i].Func == "Max" && ms[i].Orig == "<" {
			pick = &ms[i]
		}
	}
	if pick == nil {
		t.Fatal("no Max '<' mutation")
	}
	r := dockerRunOne(t, dockerBatch(t, files, *pick))
	if r.BuildError != "" {
		t.Fatalf("BuildError: %s", r.BuildError)
	}
	for _, tr := range r.Tests {
		if tr.Key() == ".:TestMax" {
			if tr.Status != "fail" || !tr.Panicked {
				t.Errorf("TestMax = %s panicked=%v\n%s", tr.Status, tr.Panicked, tr.Output)
			}
		} else if tr.Status != "pass" {
			t.Errorf("%s = %s\n%s", tr.Key(), tr.Status, tr.Output)
		}
	}
}

func TestDockerNoNetwork(t *testing.T) {
	dockerRunner(t)
	r := dockerRunOne(t, dockerBatch(t, loadDir(t, "testdata/net")))
	if r.BuildError != "" || len(r.Tests) != 1 {
		t.Fatalf("BuildError=%q tests=%+v", r.BuildError, r.Tests)
	}
	if r.Tests[0].Status != "fail" {
		t.Errorf("network reachable from sandbox: %+v", r.Tests[0])
	}
}

func TestDockerRunsAsNobody(t *testing.T) {
	dockerRunner(t)
	r := dockerRunOne(t, dockerBatch(t, loadDir(t, "testdata/whoami")))
	if r.BuildError != "" || len(r.Tests) != 1 {
		t.Fatalf("BuildError=%q tests=%+v", r.BuildError, r.Tests)
	}
	if r.Tests[0].Status != "pass" {
		t.Errorf("not running as 65534: %+v", r.Tests[0])
	}
}

func TestDockerTimeoutKillsContainer(t *testing.T) {
	d := dockerRunner(t)
	b := dockerBatch(t, loadDir(t, "testdata/loop"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := d.Run(ctx, b); err == nil {
		t.Fatal("Run returned nil error after cancel")
	}
	time.Sleep(2 * time.Second)
	out, err := exec.Command("docker", "ps", "-q", "--filter", "name=lcme-").Output()
	if err != nil {
		t.Fatal(err)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		t.Errorf("containers still running: %s", s)
	}
}

func TestDockerGoVersion(t *testing.T) {
	v, err := dockerRunner(t).GoVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v, "go1.") {
		t.Errorf("GoVersion = %q", v)
	}
}

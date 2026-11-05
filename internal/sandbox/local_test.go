package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"lcme/internal/mutate"
)

var runnerBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lcme-runner-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	runnerBin = filepath.Join(dir, "lcme-runner")
	if out, err := exec.Command("go", "build", "-o", runnerBin, "lcme/cmd/runner").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build runner: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// loadDir reads every regular file under dir into a map keyed by slash path.
func loadDir(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// variant writes files (with ms applied) as variant "v1" of a fresh batch.
func variant(t *testing.T, files map[string][]byte, ms ...mutate.Mutation) Batch {
	t.Helper()
	if len(ms) > 0 {
		var err error
		if files, err = mutate.Apply(files, ms); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	if err := WriteVariant(dir, "v1", files); err != nil {
		t.Fatal(err)
	}
	return Batch{Dir: dir, IDs: []string{"v1"}}
}

func runOne(t *testing.T, b Batch) VariantResult {
	t.Helper()
	res, err := NewLocal(runnerBin).Run(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1", len(res))
	}
	return res[0]
}

func textkit(t *testing.T) map[string][]byte { return loadDir(t, "../../examples/textkit") }

func TestLocalBaselineAllPass(t *testing.T) {
	r := runOne(t, variant(t, textkit(t)))
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
	if !r.Passed(".:TestMax") || r.Passed(".:Nope") || !r.AllPassed(r.Keys()) {
		t.Errorf("helpers wrong: %v", r.Keys())
	}
}

func TestLocalPanicIsolated(t *testing.T) {
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
	r := runOne(t, variant(t, files, *pick))
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
	if len(r.Tests) != 5 {
		t.Errorf("tests = %d, want 5", len(r.Tests))
	}
}

func TestLocalBuildError(t *testing.T) {
	files := textkit(t)
	files["textkit.go"] = bytes.Replace(files["textkit.go"], []byte("return m"), []byte("return x"), 1)
	r := runOne(t, variant(t, files))
	if r.BuildError == "" || len(r.Tests) != 0 {
		t.Fatalf("BuildError=%q tests=%d", r.BuildError, len(r.Tests))
	}
	if len(r.BuildError) > OutputCap {
		t.Errorf("BuildError too long: %d", len(r.BuildError))
	}
}

func TestLocalTimeout(t *testing.T) {
	start := time.Now()
	r := runOne(t, variant(t, loadDir(t, "testdata/loop")))
	if r.BuildError != "" {
		t.Fatalf("BuildError: %s", r.BuildError)
	}
	if len(r.Tests) != 1 || r.Tests[0].Status != "timeout" {
		t.Fatalf("tests = %+v", r.Tests)
	}
	if d := time.Since(start); d > 8*time.Second {
		t.Errorf("Run took %v", d)
	}
}

func TestLocalOutputCapped(t *testing.T) {
	files := map[string][]byte{
		"go.mod": []byte("module big\n\ngo 1.22\n"),
		"big.go": []byte("package big\n"),
		"big_test.go": []byte(`package big

import (
	"os"
	"strings"
	"testing"
)

func TestBig(t *testing.T) {
	os.Stdout.WriteString(strings.Repeat("x", 1<<20))
	t.Fatal("boom")
}
`),
	}
	r := runOne(t, variant(t, files))
	if r.BuildError != "" || len(r.Tests) != 1 {
		t.Fatalf("BuildError=%q tests=%+v", r.BuildError, r.Tests)
	}
	tr := r.Tests[0]
	if tr.Status != "fail" || len(tr.Output) == 0 || len(tr.Output) > OutputCap {
		t.Errorf("status=%s len(output)=%d", tr.Status, len(tr.Output))
	}
}

func TestLaterPackageBuildErrorClearsTests(t *testing.T) {
	files := map[string][]byte{
		"go.mod":      []byte("module two\n\ngo 1.22\n"),
		"a_test.go":   []byte("package two\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n"),
		"a.go":        []byte("package two\n"),
		"z/z.go":      []byte("package z\n\nvar X int = \"s\"\n"),
		"z/z_test.go": []byte("package z\n\nimport \"testing\"\n\nfunc TestZ(t *testing.T) {}\n"),
	}
	r := runOne(t, variant(t, files))
	if r.BuildError == "" || len(r.Tests) != 0 {
		t.Fatalf("BuildError=%q tests=%+v", r.BuildError, r.Tests)
	}
}

func TestLocalReadOnlyDir(t *testing.T) {
	b := variant(t, textkit(t))
	dir := filepath.Join(b.Dir, "v1")
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			defer os.Chmod(p, 0555)
		}
		return nil
	})
	t.Cleanup(func() {
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if d != nil && d.IsDir() {
				os.Chmod(p, 0755)
			}
			return nil
		})
	})
	r := runOne(t, b)
	if r.BuildError != "" || len(r.Tests) != 5 {
		t.Fatalf("BuildError=%q tests=%d", r.BuildError, len(r.Tests))
	}
}

func TestWriteVariantModesAndEscape(t *testing.T) {
	dir := t.TempDir()
	if err := WriteVariant(dir, "a", map[string][]byte{"sub/x.go": []byte("x")}); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]os.FileMode{"a": 0755, "a/sub": 0755, "a/sub/x.go": 0644} {
		fi, err := os.Stat(filepath.Join(dir, p))
		if err != nil || fi.Mode().Perm() != want {
			t.Errorf("%s: %v %v", p, fi, err)
		}
	}
	if err := WriteVariant(dir, "b", map[string][]byte{"../evil": nil}); err == nil {
		t.Error("path escape accepted")
	}
}

func TestParseResultsLimit(t *testing.T) {
	in := `{"id":"a"}` + "\n" + `{"id":"b"}` + "\n"
	rs, err := ParseResults(strings.NewReader(in), 1<<20)
	if err != nil || len(rs) != 2 || rs[1].ID != "b" {
		t.Fatalf("%v %+v", err, rs)
	}
	if _, err := ParseResults(strings.NewReader(in), 10); err == nil {
		t.Error("limit not enforced")
	}
}

func tinyProject(testSrc, extraSrc string) map[string][]byte {
	return map[string][]byte{
		"go.mod":       []byte("module tiny\n\ngo 1.22\n"),
		"tiny.go":      []byte("package tiny\n" + extraSrc),
		"tiny_test.go": []byte("package tiny\n\n" + testSrc),
	}
}

func TestLocalDurationRecorded(t *testing.T) {
	r := runOne(t, variant(t, textkit(t)))
	if r.DurationMs <= 0 {
		t.Errorf("DurationMs = %d", r.DurationMs)
	}
}

func TestLocalInitHangBounded(t *testing.T) {
	// Discovery does not execute init. Each known test still has a bounded process.
	files := tinyProject("import \"testing\"\n\nfunc TestX(t *testing.T) {}\n", "\nfunc init() {\n\tfor {\n\t}\n}\n")
	start := time.Now()
	r := runOne(t, variant(t, files))
	if r.BuildError != "" {
		t.Fatalf("BuildError: %s", r.BuildError)
	}
	if len(r.Tests) != 1 || r.Tests[0].Status != StatusTimeout || r.Tests[0].Name != "TestX" {
		t.Fatalf("tests = %+v", r.Tests)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("Run took %v", d)
	}
}

func TestFilterEnv(t *testing.T) {
	got := FilterEnv([]string{"PATH=/bin", "OPENROUTER_API_KEY=k", "LCME_TOKEN=x", "HOME=/h"})
	joined := strings.Join(got, "\n")
	for _, bad := range []string{"OPENROUTER_API_KEY", "LCME_TOKEN"} {
		if strings.Contains(joined, bad) {
			t.Errorf("%s leaked: %v", bad, got)
		}
	}
	for _, want := range []string{"PATH=/bin", "HOME=/h", "GOTOOLCHAIN=local", "GOPROXY=off", "GOFLAGS=-mod=readonly"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %s: %v", want, got)
		}
	}
}

func TestLocalHidesSecrets(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-secret")
	t.Setenv("LCME_SECRET", "s")
	files := tinyProject(`import (
	"os"
	"testing"
)

func TestEnv(t *testing.T) {
	if os.Getenv("OPENROUTER_API_KEY") != "" || os.Getenv("LCME_SECRET") != "" {
		t.Fatal("secret visible")
	}
}
`, "")
	r := runOne(t, variant(t, files))
	if r.BuildError != "" || len(r.Tests) != 1 || r.Tests[0].Status != StatusPass {
		t.Fatalf("%+v", r)
	}
}

func TestLocalKillsBackgroundProcess(t *testing.T) {
	files := tinyProject(`import (
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func TestSpawn(t *testing.T) {
	c := exec.Command("sleep", "60")
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	os.WriteFile("../pid", []byte(strconv.Itoa(c.Process.Pid)), 0o644)
}
`, "")
	b := variant(t, files)
	r := runOne(t, b)
	if len(r.Tests) != 1 || r.Tests[0].Status != StatusPass {
		t.Fatalf("%+v", r)
	}
	data, err := os.ReadFile(filepath.Join(b.Dir, "pid"))
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	fmt.Sscan(string(data), &pid)
	for i := 0; i < 40; i++ {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL)
	t.Error("background process survived")
}

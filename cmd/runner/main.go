// Command runner (lcme-runner) builds each variant's test binaries and runs
// every test in its own process, printing one JSON VariantResult per line.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"lcme/internal/sandbox"
)

const (
	buildTimeout = 60 * time.Second
	// tailSize is how much trailing output is kept for marker detection.
	tailSize = 64 * 1024
)

func main() {
	dir := flag.String("dir", "", "batch directory")
	ids := flag.String("ids", "", "comma-separated variant ids")
	testTimeout := flag.Duration("test-timeout", sandbox.TestTimeout, "per-test timeout")
	flag.Parse()
	if *dir == "" || *ids == "" {
		fmt.Fprintln(os.Stderr, "usage: lcme-runner -dir <batch> -ids a,b,c [-test-timeout 2s]")
		os.Exit(2)
	}
	tmp, err := os.MkdirTemp("", "lcme-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	enc := json.NewEncoder(os.Stdout)
	for _, id := range strings.Split(*ids, ",") {
		res := runVariant(filepath.Join(*dir, id), id, tmp, *testTimeout)
		if err := enc.Encode(res); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// tail keeps the last n bytes written to it.
type tail struct {
	n   int
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > 2*t.n {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-t.n:]...)
	}
	return len(p), nil
}

func (t *tail) bytes() []byte {
	if len(t.buf) > t.n {
		return t.buf[len(t.buf)-t.n:]
	}
	return t.buf
}

// capOutput keeps the last OutputCap bytes of b as valid UTF-8.
func capOutput(b []byte) string {
	if len(b) > sandbox.OutputCap {
		b = b[len(b)-sandbox.OutputCap:]
	}
	return strings.ToValidUTF8(string(b), "")
}

func runVariant(dir, id, tmp string, testTimeout time.Duration) (res sandbox.VariantResult) {
	start := time.Now()
	res = sandbox.VariantResult{ID: id, Tests: []sandbox.TestResult{}}
	defer func() { res.DurationMs = time.Since(start).Milliseconds() }()
	absoluteDir, err := filepath.Abs(dir)
	if err != nil {
		res.InfrastructureError = "Не удалось определить каталог тестового проекта."
		return res
	}
	dir = absoluteDir

	pkgs, err := testPackages(dir)
	if err != nil {
		res.InfrastructureError = infrastructureFailure(err, err.Error())
		if res.InfrastructureError == "" {
			res.BuildError = capOutput([]byte(err.Error()))
		}
		return res
	}
	binDir := filepath.Join(tmp, id)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		res.InfrastructureError = "Не удалось подготовить временный каталог тестов."
		return res
	}
	defer os.RemoveAll(binDir)
	for n, pkg := range pkgs {
		bin := filepath.Join(binDir, strconv.Itoa(n)+".test")
		names, overlay, err := discoverAndInstrument(dir, pkg, bin)
		if err != nil {
			res.InfrastructureError = infrastructureFailure(err, err.Error())
			if res.InfrastructureError == "" {
				res.BuildError = capOutput([]byte(err.Error()))
			}
			res.Tests = []sandbox.TestResult{}
			return res
		}
		if out, err := goBuild(dir, pkg, bin, overlay); err != nil {
			if problem := infrastructureFailure(err, string(out)); problem != "" {
				res.InfrastructureError = problem
				res.Tests = []sandbox.TestResult{}
				return res
			}
			res.BuildError = capOutput(out)
			if res.BuildError == "" {
				res.BuildError = err.Error()
			}
			res.Tests = []sandbox.TestResult{}
			return res
		}
		for _, name := range names {
			result := runTestTarget(dir, pkg, bin, name.RunName, name.Name, testTimeout)
			result.Name = name.Name
			res.Tests = append(res.Tests, result)
		}
		// Both the instrumented source and compiled package are no longer
		// needed; retaining every package/variant can fill the shared tmpfs.
		_ = os.Remove(bin)
		_ = os.RemoveAll(filepath.Dir(overlay))
	}
	return res
}

func infrastructureFailure(err error, output string) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "Превышено время подготовки тестовой среды."
	case errors.Is(err, syscall.ENOSPC) || strings.Contains(output, "no space left on device"):
		return "Недостаточно временного места в среде запуска тестов."
	case errors.Is(err, syscall.ENOMEM) || strings.Contains(output, "cannot allocate memory"):
		return "Недостаточно памяти в среде запуска тестов."
	default:
		return ""
	}
}

// Ask the same toolchain that builds the tests for the current module's
// build-selected packages. This excludes nested modules and inactive tags.
func testPackages(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-json", "./...")
	cmd.Dir = root
	sandbox.SetProcessGroup(cmd)
	output := &boundedBuffer{limit: 4 << 20}
	errout := &tail{n: sandbox.OutputCap}
	cmd.Stdout, cmd.Stderr = output, errout
	err := cmd.Run()
	sandbox.KillGroup(cmd)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("list packages: %w: %s", err, errout.bytes())
	}
	if output.overflow {
		return nil, fmt.Errorf("package metadata exceeds 4 MiB")
	}
	var pkgs []string
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	for {
		var info struct {
			Dir                       string
			TestGoFiles, XTestGoFiles []string
		}
		if err := decoder.Decode(&info); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		if len(info.TestGoFiles)+len(info.XTestGoFiles) == 0 {
			continue
		}
		rel, err := filepath.Rel(root, info.Dir)
		if err != nil {
			return nil, err
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("test package outside module")
		}
		pkgs = append(pkgs, filepath.ToSlash(rel))
	}
	sort.Strings(pkgs)
	return pkgs, nil
}

func goBuild(dir, pkg, bin, overlay string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-c", "-overlay", overlay, "-o", bin, "./"+pkg)
	cmd.Dir = dir
	sandbox.SetProcessGroup(cmd)
	t := &tail{n: sandbox.OutputCap}
	cmd.Stdout, cmd.Stderr = t, t
	err := cmd.Run()
	sandbox.KillGroup(cmd)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return t.bytes(), err
}

func runTest(dir, pkg, bin, name string, testTimeout time.Duration) sandbox.TestResult {
	return runTestTarget(dir, pkg, bin, name, name, testTimeout)
}

func runTestTarget(dir, pkg, bin, name, target string, testTimeout time.Duration) sandbox.TestResult {
	tr := sandbox.TestResult{Pkg: pkg, Name: name, Status: sandbox.StatusFail}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout+3*time.Second)
	defer cancel()
	reader, writer, err := os.Pipe()
	if err != nil {
		tr.Output = capOutput([]byte(err.Error()))
		return tr
	}
	defer reader.Close()
	defer writer.Close()
	deadline, _ := ctx.Deadline()
	if err := reader.SetReadDeadline(deadline); err != nil {
		tr.Output = capOutput([]byte(err.Error()))
		return tr
	}
	type completionRead struct {
		data []byte
		err  error
	}
	received := make(chan completionRead, 1)
	go func() {
		// Bound both memory and the lifetime of inherited completion FDs.
		data, err := io.ReadAll(io.LimitReader(reader, 4097))
		reader.Close()
		received <- completionRead{data, err}
	}()
	cmd := exec.CommandContext(ctx, bin, "-test.run", "^"+regexp.QuoteMeta(name)+"$", "-test.count", "1", "-test.timeout", testTimeout.String(), "-test.paniconexit0")
	cmd.Env = append(os.Environ(), "LCME_TEST_TARGET="+target)
	cmd.Dir = filepath.Join(dir, filepath.FromSlash(pkg))
	cmd.ExtraFiles = []*os.File{writer}
	sandbox.SetProcessGroup(cmd)
	t := &tail{n: tailSize}
	cmd.Stdout, cmd.Stderr = t, t
	err = cmd.Run()
	sandbox.KillGroup(cmd)
	writer.Close()
	evidence := <-received
	var completed struct {
		Name            string
		Failed, Skipped *bool
	}
	confirmed := evidence.err == nil && len(evidence.data) <= 4096 &&
		json.Unmarshal(evidence.data, &completed) == nil && completed.Name == name &&
		completed.Failed != nil && !*completed.Failed && completed.Skipped != nil && !*completed.Skipped
	out := t.bytes()
	timedOut := ctx.Err() != nil || bytes.Contains(out, []byte("panic: test timed out"))
	switch {
	case timedOut:
		tr.Status = sandbox.StatusTimeout
	case err == nil && confirmed:
		tr.Status = sandbox.StatusPass
	default:
		tr.Status = sandbox.StatusFail
		tr.Panicked = bytes.Contains(out, []byte("panic: "))
	}
	tr.Output = capOutput(out)
	if err == nil && !confirmed && !timedOut {
		tr.Output = capOutput(append(out, []byte("\nlcme: requested test did not report successful completion\n")...))
	}
	return tr
}

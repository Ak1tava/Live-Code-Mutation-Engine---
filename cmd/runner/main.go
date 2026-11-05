// Command runner (lcme-runner) builds each variant's test binaries and runs
// every test in its own process, printing one JSON VariantResult per line.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

	pkgs, err := testPackages(dir)
	if err != nil {
		res.BuildError = capOutput([]byte(err.Error()))
		return res
	}
	binDir := filepath.Join(tmp, id)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		res.BuildError = capOutput([]byte(err.Error()))
		return res
	}
	for n, pkg := range pkgs {
		bin := filepath.Join(binDir, strconv.Itoa(n)+".test")
		names, overlay, err := discoverAndInstrument(dir, pkg, bin)
		if err != nil {
			res.BuildError = capOutput([]byte(err.Error()))
			res.Tests = []sandbox.TestResult{}
			return res
		}
		if out, err := goBuild(dir, pkg, bin, overlay); err != nil {
			res.BuildError = capOutput(out)
			if res.BuildError == "" {
				res.BuildError = err.Error()
			}
			res.Tests = []sandbox.TestResult{}
			return res
		}
		for _, name := range names {
			res.Tests = append(res.Tests, runTest(dir, pkg, bin, name, testTimeout))
		}
	}
	return res
}

// testPackages returns the sorted package dirs (relative, "." for the root)
// that contain *_test.go files.
func testPackages(root string) ([]string, error) {
	set := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, "_test.go") {
			rel, err := filepath.Rel(root, filepath.Dir(p))
			if err != nil {
				return err
			}
			set[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	pkgs := make([]string, 0, len(set))
	for p := range set {
		pkgs = append(pkgs, p)
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
	return t.bytes(), err
}

func runTest(dir, pkg, bin, name string, testTimeout time.Duration) sandbox.TestResult {
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

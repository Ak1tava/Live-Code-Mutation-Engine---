package selector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lcme/internal/mutate"
	"lcme/internal/sandbox"
)

var runnerBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lcme-runner-selector")
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

var abc = []string{"A", "B", "C"}

// mk builds the 8 variant results for tests A, B, C from a failure predicate.
func mk(fails func(mask int, test string) bool) [8]sandbox.VariantResult {
	var r [8]sandbox.VariantResult
	for mask := range r {
		for _, name := range abc {
			st := sandbox.StatusPass
			if fails(mask, name) {
				st = sandbox.StatusFail
			}
			r[mask].Tests = append(r[mask].Tests, sandbox.TestResult{Pkg: ".", Name: name, Status: st})
		}
	}
	return r
}

func keys(names ...string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "." + ":" + n
	}
	return out
}

func eq(a, b []string) bool { return strings.Join(a, ",") == strings.Join(b, ",") }

func TestIndependentOK(t *testing.T) {
	bit := map[string]int{"A": 0, "B": 1, "C": 2}
	r := mk(func(mask int, test string) bool { return mask&(1<<bit[test]) != 0 })
	killing, ok := Independent(r, keys(abc...))
	if !ok {
		t.Fatal("ok = false")
	}
	for i, n := range abc {
		if !eq(killing[i], keys(n)) {
			t.Errorf("killing[%d] = %v, want %v", i, killing[i], keys(n))
		}
	}
}

func TestIndependentMasked(t *testing.T) {
	bit := map[string]int{"A": 0, "B": 1, "C": 2}
	r := mk(func(mask int, test string) bool {
		if test == "B" && mask&1 != 0 { // B also fails whenever mutation 0 is present
			return true
		}
		return mask&(1<<bit[test]) != 0
	})
	if _, ok := Independent(r, keys(abc...)); ok {
		t.Error("ok = true, want false")
	}
}

func TestIndependentSharedTest(t *testing.T) {
	r := mk(func(mask int, test string) bool { return test == "A" && mask != 0 })
	if _, ok := Independent(r, keys(abc...)); ok {
		t.Error("ok = true, want false")
	}
}

func TestIndependentBuildError(t *testing.T) {
	bit := map[string]int{"A": 0, "B": 1, "C": 2}
	r := mk(func(mask int, test string) bool { return mask&(1<<bit[test]) != 0 })
	r[3] = sandbox.VariantResult{BuildError: "boom"}
	if _, ok := Independent(r, keys(abc...)); ok {
		t.Error("ok = true, want false")
	}
}

func TestScoreOrder(t *testing.T) {
	c := func(k mutate.Kind, fn string, panicked bool) Candidate {
		return Candidate{M: mutate.Mutation{Kind: k, Func: fn}, Panicked: panicked}
	}
	b, a, k := Score(c(mutate.Boundary, "f", false)), Score(c(mutate.Arith, "f", false)), Score(c(mutate.Constant, "f", false))
	if !(b > a && a > k) {
		t.Errorf("boundary=%d arith=%d constant=%d, want strictly decreasing", b, a, k)
	}
	if bp := Score(c(mutate.Boundary, "f", true)); bp >= b {
		t.Errorf("panicked boundary %d >= plain boundary %d", bp, b)
	}
	if m := Score(c(mutate.Boundary, "main", false)); m >= b {
		t.Errorf("main boundary %d >= non-main boundary %d", m, b)
	}
}

type recordingCandidateRunner struct {
	t       *testing.T
	batches []int
	seen    []string
	fail    bool
}

func (r *recordingCandidateRunner) GoVersion(context.Context) (string, error) {
	return "", nil
}

func (r *recordingCandidateRunner) Run(_ context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
	r.batches = append(r.batches, len(b.IDs))
	r.seen = append(r.seen, b.IDs...)
	var bytes int64
	err := filepath.WalkDir(b.Dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		bytes += info.Size()
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(b.IDs) > 16 || bytes > 32<<20 {
		r.t.Errorf("candidate batch contains %d variants and %d bytes", len(b.IDs), bytes)
	}
	out := make([]sandbox.VariantResult, 0, len(b.IDs))
	status := sandbox.StatusPass
	if r.fail {
		status = sandbox.StatusFail
	}
	for _, id := range b.IDs {
		out = append(out, sandbox.VariantResult{ID: id, Tests: []sandbox.TestResult{
			{Pkg: ".", Name: "A", Status: status},
		}})
	}
	return out, nil
}

func TestEvaluateBoundsCandidateBatches(t *testing.T) {
	for _, tc := range []struct {
		name        string
		candidates  int
		paddingMiB  int
		wantBatches []int
	}{
		{"variant count", 18, 0, []int{16, 2}},
		{"staged bytes", 2, 17, []int{1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "package p\nfunc f() {\n" + strings.Repeat("_ = 1\n", tc.candidates) + "}\n"
			goFiles := map[string][]byte{"p.go": []byte(source)}
			files := map[string][]byte{"p.go": goFiles["p.go"]}
			// Share the padding in memory, but account for every file staged
			// in each variant. The fake runner never compiles the fixture.
			padding := make([]byte, 1<<20)
			for i := 0; i < tc.paddingMiB; i++ {
				files[fmt.Sprintf("padding-%d", i)] = padding
			}
			r := &recordingCandidateRunner{t: t}
			work := t.TempDir()
			var progress []Progress
			cands, _, stats, err := evaluate(context.Background(), files, goFiles, r, work, keys("A"), func(p Progress) {
				progress = append(progress, p)
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(cands) != 0 || stats.Candidates != tc.candidates || stats.Survived != tc.candidates {
				t.Errorf("candidates=%v stats=%+v", cands, stats)
			}
			if !slices.Equal(r.batches, tc.wantBatches) {
				t.Errorf("batch sizes = %v, want %v", r.batches, tc.wantBatches)
			}
			var wantProgress []Progress
			done := 0
			for _, count := range tc.wantBatches {
				done += count
				wantProgress = append(wantProgress, Progress{"candidates", done, tc.candidates})
			}
			if !slices.Equal(progress, wantProgress) {
				t.Errorf("progress = %v, want %v", progress, wantProgress)
			}
			for i, id := range r.seen {
				if want := fmt.Sprintf("c%d", i+1); id != want {
					t.Errorf("candidate[%d] = %s, want %s", i, id, want)
				}
			}
			if entries, err := os.ReadDir(work); err != nil || len(entries) != 0 {
				t.Errorf("work directory contains %v, err=%v", entries, err)
			}
		})
	}
}

func TestEvaluateRetainsStableTopCandidatesAcrossBatches(t *testing.T) {
	const source = "package p\nfunc f(n int) int {\n" +
		"_ = 1\n_ = 1\n_ = 1\n_ = 1\n_ = 1\n_ = 1\n_ = 1\n_ = 1\n" +
		"_ = 1\n_ = 1\n_ = 1\n_ = 1\n_ = 1\n_ = 1\n_ = 1\n_ = 1\n" +
		"return n + 1\n}\n"
	files := map[string][]byte{"p.go": []byte(source)}
	r := &recordingCandidateRunner{t: t, fail: true}
	cands, kept, stats, err := evaluate(context.Background(), files, files, r, t.TempDir(), keys("A"), func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 12 || len(kept) != 12 || stats.Killed != 18 {
		t.Fatalf("candidates=%d kept=%d stats=%+v", len(cands), len(kept), stats)
	}
	want := []string{"c17", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9", "c10", "c11"}
	for i, c := range cands {
		if c.M.ID != want[i] || kept[c.M.ID].ID != c.M.ID {
			t.Errorf("candidate[%d] = %s, want %s and matching result", i, c.M.ID, want[i])
		}
	}
}

// ---- integration ----

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

func goSources(files map[string][]byte) map[string][]byte {
	out := map[string][]byte{}
	for n, b := range files {
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			out[n] = b
		}
	}
	return out
}

func prepare(t *testing.T, dir string) (map[string][]byte, *Plan, string, error) {
	t.Helper()
	files := loadDir(t, dir)
	work := filepath.Join(t.TempDir(), "work")
	plan, err := Prepare(context.Background(), files, goSources(files), sandbox.NewLocal(runnerBin),
		work, DefaultOptions, nil)
	return files, plan, work, err
}

// runWith applies ms to files and runs the project once.
func runWith(t *testing.T, files map[string][]byte, ms ...mutate.Mutation) sandbox.VariantResult {
	t.Helper()
	if len(ms) > 0 {
		var err error
		if files, err = mutate.Apply(files, ms); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	if err := sandbox.WriteVariant(dir, "v1", files); err != nil {
		t.Fatal(err)
	}
	res, err := sandbox.NewLocal(runnerBin).Run(context.Background(), sandbox.Batch{Dir: dir, IDs: []string{"v1"}})
	if err != nil || len(res) != 1 {
		t.Fatalf("run: %v (%d results)", err, len(res))
	}
	return res[0]
}

func failed(v sandbox.VariantResult) []string {
	var out []string
	for _, tr := range v.Tests {
		if tr.Status != sandbox.StatusPass {
			out = append(out, tr.Key())
		}
	}
	return out
}

func diffLines(a, b map[string][]byte) int {
	n := 0
	for name, ab := range a {
		al := bytes.Split(ab, []byte("\n"))
		bl := bytes.Split(b[name], []byte("\n"))
		if len(al) != len(bl) {
			n += 1000
			continue
		}
		for i := range al {
			if !bytes.Equal(al[i], bl[i]) {
				n++
			}
		}
	}
	return n
}

func TestPrepareTextkit(t *testing.T) {
	files, plan, work, err := prepare(t, "../../examples/textkit")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Mutations) != 3 || len(plan.KillingTests) != 3 {
		t.Fatalf("mutations=%d killing=%d", len(plan.Mutations), len(plan.KillingTests))
	}
	funcs := map[string]bool{}
	for i, m := range plan.Mutations {
		funcs[m.Func] = true
		if want := fmt.Sprintf("m%d", i+1); m.ID != want {
			t.Errorf("ID = %s, want %s", m.ID, want)
		}
		if len(plan.KillingTests[i]) == 0 {
			t.Errorf("KillingTests[%d] empty", i)
		}
		if i > 0 {
			p := plan.Mutations[i-1]
			if p.File > m.File || (p.File == m.File && p.Start >= m.Start) {
				t.Errorf("mutations %d and %d not in (file, offset) order", i-1, i)
			}
		}
	}
	if len(funcs) != 3 {
		t.Errorf("distinct funcs = %d, want 3: %v", len(funcs), funcs)
	}
	if n := diffLines(files, plan.Mutated); n != 3 {
		t.Errorf("Mutated differs in %d lines, want 3", n)
	}
	if len(plan.BaselineTests) != 5 {
		t.Errorf("BaselineTests = %v", plan.BaselineTests)
	}
	if plan.Stats.BuildErrors == 0 {
		t.Errorf("BuildErrors = 0, want > 0 (string + in Capitalize); stats %+v", plan.Stats)
	}
	if plan.Stats.Killed == 0 || plan.Stats.Candidates < plan.Stats.Killed+plan.Stats.BuildErrors {
		t.Errorf("inconsistent stats %+v", plan.Stats)
	}

	// Each mutation alone: no panic, and exactly KillingTests[i] fail.
	for i, m := range plan.Mutations {
		v := runWith(t, files, m)
		if v.BuildError != "" {
			t.Fatalf("m%d does not build: %s", i+1, v.BuildError)
		}
		for _, tr := range v.Tests {
			if tr.Panicked {
				t.Errorf("m%d: %s panicked", i+1, tr.Key())
			}
		}
		f := failed(v)
		for _, k := range plan.KillingTests[i] {
			found := false
			for _, x := range f {
				found = found || x == k
			}
			if !found {
				t.Errorf("m%d alone: %s did not fail (failed: %v)", i+1, k, f)
			}
		}
		// Tests killing a different mutation must not fail here.
		for j, ks := range plan.KillingTests {
			if j == i {
				continue
			}
			for _, k := range ks {
				for _, x := range f {
					if x == k {
						t.Errorf("m%d alone made KillingTests[%d] test %s fail", i+1, j, k)
					}
				}
			}
		}
	}
	// All three together: every killing test fails.
	all := failed(runWith(t, files, plan.Mutations...))
	for i, ks := range plan.KillingTests {
		for _, k := range ks {
			ok := false
			for _, x := range all {
				ok = ok || x == k
			}
			if !ok {
				t.Errorf("all mutations: KillingTests[%d] test %s passed", i, k)
			}
		}
	}

	// Batch directories are removed.
	if ents, err := os.ReadDir(work); err != nil || len(ents) != 0 {
		t.Errorf("workDir not clean: %v %v", ents, err)
	}
}

func TestPrepareBaselineRed(t *testing.T) {
	_, _, _, err := prepare(t, "testdata/red")
	if !errors.Is(err, ErrBaselineRed) {
		t.Fatalf("err = %v, want ErrBaselineRed", err)
	}
	if !strings.Contains(err.Error(), "TestDouble") {
		t.Errorf("error does not list failing test: %v", err)
	}
}

func TestPrepareNoTriple(t *testing.T) {
	_, _, _, err := prepare(t, "testdata/onefunc")
	if !errors.Is(err, ErrNoTriple) {
		t.Fatalf("err = %v, want ErrNoTriple", err)
	}
}

func TestPrepareCancelled(t *testing.T) {
	files := loadDir(t, "../../examples/textkit")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	work := filepath.Join(t.TempDir(), "work")
	_, err := Prepare(ctx, files, goSources(files), sandbox.NewLocal(runnerBin), work, DefaultOptions, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestPrepareProgress(t *testing.T) {
	files := loadDir(t, "../../examples/textkit")
	var stages []string
	_, err := Prepare(context.Background(), files, goSources(files), sandbox.NewLocal(runnerBin),
		filepath.Join(t.TempDir(), "work"), DefaultOptions, func(p Progress) { stages = append(stages, p.Stage) })
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) < 4 || stages[0] != "baseline" || stages[1] != "candidates" || stages[len(stages)-1] != "done" {
		t.Errorf("stages = %v", stages)
	}
}

func TestIndependentMissingTest(t *testing.T) {
	bit := map[string]int{"A": 0, "B": 1, "C": 2}
	r := mk(func(mask int, test string) bool { return mask&(1<<bit[test]) != 0 })
	// Mask 7 lost test A and gained a synthetic (init) failure.
	r[7].Tests = []sandbox.TestResult{
		{Pkg: ".", Name: "B", Status: sandbox.StatusFail},
		{Pkg: ".", Name: "C", Status: sandbox.StatusFail},
		{Pkg: ".", Name: "(init)", Status: sandbox.StatusFail},
	}
	if _, ok := Independent(r, keys(abc...)); ok {
		t.Error("ok = true, want false")
	}
}

// initRunner delegates to a real runner but replaces the result of the given
// variants with a lone synthetic "(init)" failure.
type initRunner struct {
	sandbox.Runner
	bad map[string]bool
}

func (f initRunner) Run(ctx context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
	res, err := f.Runner.Run(ctx, b)
	for i := range res {
		if f.bad[res[i].ID] {
			res[i].Tests = []sandbox.TestResult{{Pkg: "pkg", Name: "(init)", Status: sandbox.StatusFail}}
		}
	}
	return res, err
}

func TestPrepareInitResultNotCandidate(t *testing.T) {
	files := loadDir(t, "../../examples/textkit")
	src := goSources(files)
	all, err := mutate.Generate(src)
	if err != nil {
		t.Fatal(err)
	}
	bad := map[string]bool{}
	for _, m := range all {
		if m.Func == "Max" {
			bad[m.ID] = true
		}
	}
	if len(bad) == 0 {
		t.Fatal("no Max candidates")
	}
	r := initRunner{Runner: sandbox.NewLocal(runnerBin), bad: bad}
	plan, err := Prepare(context.Background(), files, src, r, filepath.Join(t.TempDir(), "work"), DefaultOptions, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range plan.Mutations {
		if m.Func == "Max" {
			t.Errorf("mutation %+v from a variant with (init) result was selected", m)
		}
	}
	if plan.Stats.BuildErrors < len(bad) {
		t.Errorf("BuildErrors = %d, want >= %d", plan.Stats.BuildErrors, len(bad))
	}
}

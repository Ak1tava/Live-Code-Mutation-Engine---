package selector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lcme/internal/mutate"
	"lcme/internal/sandbox"
)

func TestBaselineErrorDoesNotExposeRunnerOutput(t *testing.T) {
	_, err := checkBaseline([]string{"b1"}, map[string]sandbox.VariantResult{
		"b1": {BuildError: "secret=/host/private/token\n" + strings.Repeat("x", 6000)},
	})
	if !errors.Is(err, ErrBaselineRed) {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "secret") || len(err.Error()) > 2048 {
		t.Fatalf("unsafe baseline diagnostic: %d bytes", len(err.Error()))
	}
}

func TestBaselineErrorBoundsFailingTests(t *testing.T) {
	v := sandbox.VariantResult{}
	for i := 0; i < 20; i++ {
		v.Tests = append(v.Tests, sandbox.TestResult{Pkg: ".", Name: fmt.Sprintf("Test%d", i), Status: sandbox.StatusFail})
	}
	v.Tests = append(v.Tests, sandbox.TestResult{Pkg: ".", Name: "Test\nunsafe", Status: sandbox.StatusFail})
	_, err := checkBaseline([]string{"b1"}, map[string]sandbox.VariantResult{"b1": v})
	var diagnostic *BaselineError
	if !errors.As(err, &diagnostic) || diagnostic.Kind != "tests" || len(diagnostic.FailingTests) != 8 || strings.Contains(err.Error(), "\n") {
		t.Fatalf("unsafe diagnostic: %v", err)
	}
}

func TestCandidateSelectionSeedAndBudget(t *testing.T) {
	var ms []mutate.Mutation
	for f := 0; f < 30; f++ {
		for i := 0; i < 41; i++ {
			ms = append(ms, mutate.Mutation{ID: fmt.Sprintf("c%d", len(ms)), File: "p.go", Func: fmt.Sprintf("F%d", f), Kind: mutate.Boundary})
		}
	}
	for _, seed := range []int64{0, 1, 2, 3} {
		opt := DefaultOptions
		opt.Seed = seed
		a, b := selectCandidates(ms, opt), selectCandidates(ms, opt)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("seed %d is not reproducible", seed)
		}
		funcs := map[string]bool{}
		for _, m := range a {
			funcs[m.Func] = true
		}
		if len(a) != 64 || len(funcs) != 30 {
			t.Fatalf("seed %d selected %d candidates from %d functions", seed, len(a), len(funcs))
		}
		opt.MaxCandidates = 5
		if got := selectCandidates(ms, opt); len(got) != 5 {
			t.Fatalf("custom budget ignored: %d", len(got))
		}
	}
	a, b := DefaultOptions, DefaultOptions
	a.Seed, b.Seed = 1, 2
	if reflect.DeepEqual(selectCandidates(ms, a), selectCandidates(ms, b)) {
		t.Fatal("distinct seeds selected identical candidates")
	}
}

type independentFilesRunner struct{ files map[string][]byte }

func (independentFilesRunner) GoVersion(context.Context) (string, error) { return "", nil }
func (r independentFilesRunner) Run(_ context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
	var out []sandbox.VariantResult
	for _, id := range b.IDs {
		v := sandbox.VariantResult{ID: id}
		for i := 0; i < 6; i++ {
			name := fmt.Sprintf("f%d.go", i)
			data, err := os.ReadFile(filepath.Join(b.Dir, id, name))
			if err != nil {
				return nil, err
			}
			status := sandbox.StatusPass
			if !bytes.Equal(data, r.files[name]) {
				status = sandbox.StatusFail
			}
			v.Tests = append(v.Tests, sandbox.TestResult{Pkg: ".", Name: fmt.Sprintf("TestF%d", i), Status: status})
		}
		out = append(out, v)
	}
	return out, nil
}

func TestPrepareSeedChangesChosenTriple(t *testing.T) {
	files := map[string][]byte{}
	for i := 0; i < 6; i++ {
		files[fmt.Sprintf("f%d.go", i)] = []byte(fmt.Sprintf("package p\nfunc F%d(n int) bool { return n < 10 }\n", i))
	}
	signatures := map[int64][]mutate.Mutation{}
	for _, seed := range []int64{0, 1, 2, 1, 0} {
		opt := DefaultOptions
		opt.Seed = seed
		plan, err := Prepare(context.Background(), files, files, independentFilesRunner{files}, t.TempDir(), opt, nil)
		if err != nil {
			t.Fatal(err)
		}
		if prior, ok := signatures[seed]; ok && !reflect.DeepEqual(prior, plan.Mutations) {
			t.Fatalf("seed %d changed on repetition", seed)
		}
		signatures[seed] = plan.Mutations
	}
	if reflect.DeepEqual(signatures[1], signatures[2]) {
		t.Fatalf("distinct seeds chose identical triples: %+v", signatures[1])
	}
}

type infrastructureRunner struct{}

func (infrastructureRunner) GoVersion(context.Context) (string, error) { return "", nil }
func (infrastructureRunner) Run(_ context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
	return []sandbox.VariantResult{{ID: b.IDs[0], InfrastructureError: "no space left on device"}}, nil
}

func TestPrepareStopsOnInfrastructureError(t *testing.T) {
	files := map[string][]byte{"p.go": []byte("package p\nfunc F() int { return 1 }\n")}
	_, err := Prepare(context.Background(), files, files, infrastructureRunner{}, t.TempDir(), DefaultOptions, nil)
	if !errors.Is(err, ErrInfrastructure) || errors.Is(err, ErrBaselineRed) {
		t.Fatalf("got %v, want infrastructure error", err)
	}
	_, _, _, err = evaluate(context.Background(), files, files, infrastructureRunner{}, t.TempDir(), keys("A"), func(Progress) {})
	if !errors.Is(err, ErrInfrastructure) {
		t.Fatalf("candidate infrastructure error discarded: %v", err)
	}
}

func TestEvaluateLimitsBudgetAcrossFunctions(t *testing.T) {
	source := "package p\nfunc Alpha() {\n" + strings.Repeat("_ = 1\n", 90) + "}\nfunc Beta() { _ = 2 }\nfunc Gamma() { _ = 3 }\n"
	files := map[string][]byte{"p.go": []byte(source)}
	r := &recordingCandidateRunner{t: t, fail: true}
	cands, kept, stats, err := evaluate(context.Background(), files, files, r, t.TempDir(), keys("A"), func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.seen) > 64 || stats.Candidates != 92 {
		t.Errorf("ran %d candidates of %d, want at most 64 of 92", len(r.seen), stats.Candidates)
	}
	funcs := map[string]bool{}
	for _, c := range cands {
		funcs[c.M.Func] = true
		if _, ok := kept[c.M.ID]; !ok {
			t.Errorf("missing result for %s", c.M.ID)
		}
	}
	if !funcs["Beta"] || !funcs["Gamma"] {
		t.Errorf("small functions were lost: %v", funcs)
	}
}

func TestRankKeepsFunctionsBeyondFirstTwelve(t *testing.T) {
	var cands []Candidate
	for i := 0; i < 12; i++ {
		cands = append(cands, Candidate{M: mutate.Mutation{Func: "Alpha", Kind: mutate.Boundary}, Failing: keys("Alpha")})
	}
	for _, name := range []string{"Beta", "Gamma"} {
		cands = append(cands, Candidate{M: mutate.Mutation{Func: name, Kind: mutate.Constant}, Failing: keys(name)})
	}
	if got := rankTriples(cands); len(got) != 12 {
		t.Fatalf("ranked %d triples, want 12 valid independent function combinations", len(got))
	}
}

func TestRunBatchBoundsCombinedVariants(t *testing.T) {
	for _, tc := range []struct {
		name              string
		count, paddingMiB int
	}{
		{"variant count", 20, 0}, {"staged bytes", 4, 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids := make([]string, tc.count)
			for i := range ids {
				ids[i] = fmt.Sprintf("t%d_7", i)
			}
			r := &recordingCandidateRunner{t: t}
			res, err := runBatch(context.Background(), r, t.TempDir(), "triples", ids, func(int) (map[string][]byte, error) {
				return map[string][]byte{"p.go": []byte("package p\n"), "padding": make([]byte, tc.paddingMiB<<20)}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(res) != tc.count {
				t.Fatalf("got %d results, want %d", len(res), tc.count)
			}
		})
	}
}

func TestPrepareFindsTripleWithLargeFirstFunction(t *testing.T) {
	var src strings.Builder
	src.WriteString("package p\nfunc Alpha(n int) int { c := 0\n")
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&src, "if n > %d { c += 1 }\n", i)
	}
	src.WriteString("return c }\nfunc Beta(n int) bool { return n > 50 }\nfunc Gamma(n int) bool { return n == 70 }\n")
	files := map[string][]byte{
		"go.mod": []byte("module p\n\ngo 1.24\n"),
		"p.go":   []byte(src.String()),
		"p_test.go": []byte(`package p
import "testing"
func TestAlpha(t *testing.T) { for n:=1; n<=12; n++ { if Alpha(n)!=n-1 { t.Fatal(n) } } }
func TestBeta(t *testing.T) { if Beta(50) || !Beta(51) { t.Fatal("Beta") } }
func TestGamma(t *testing.T) { if !Gamma(70) || Gamma(71) { t.Fatal("Gamma") } }
`),
	}
	plan, err := Prepare(context.Background(), files, goSources(files), sandbox.NewLocal(runnerBin), t.TempDir(), DefaultOptions, nil)
	if err != nil {
		t.Fatalf("valid triple rejected: %v", err)
	}
	funcs := map[string]bool{}
	for i, m := range plan.Mutations {
		funcs[m.Func] = true
		if len(plan.KillingTests[i]) == 0 {
			t.Errorf("mutation %s lacks a killing test", m.ID)
		}
	}
	if len(plan.Mutations) != 3 || !funcs["Alpha"] || !funcs["Beta"] || !funcs["Gamma"] {
		t.Fatalf("invalid triple: %+v", plan.Mutations)
	}
}

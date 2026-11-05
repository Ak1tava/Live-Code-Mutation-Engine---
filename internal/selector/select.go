package selector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"lcme/internal/mutate"
	"lcme/internal/sandbox"
)

// Options tunes Prepare.
type Options struct{ BaselineRuns, MaxTriples, TriplesPerBatch int }

// DefaultOptions are the values used in production.
var DefaultOptions = Options{BaselineRuns: 3, MaxTriples: 30, TriplesPerBatch: 5}

// Progress is reported after every sandbox batch.
type Progress struct {
	Stage       string // "baseline" | "candidates" | "triples" | "done"
	Done, Total int
}

// Stats summarises the selection process.
type Stats struct{ Candidates, Killed, Survived, BuildErrors, TriplesChecked int }

// Plan is the prepared exercise.
type Plan struct {
	Mutations     []mutate.Mutation // IDs m1..m3 in (file, offset) order
	KillingTests  [][]string        // [i] fails exactly when mutation i is applied
	BaselineTests []string
	Mutated       map[string][]byte `json:"-"`
	Stats         Stats
}

var (
	ErrBaselineRed = errors.New("baseline is not green")
	ErrNoTriple    = errors.New("no independent triple")
)

// runBatch writes the variants, runs them and removes the batch directory.
// Results are returned keyed by variant ID.
func runBatch(ctx context.Context, r sandbox.Runner, workDir, name string,
	ids []string, variants map[string]map[string][]byte) (map[string]sandbox.VariantResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(workDir, name+"-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0o755); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := sandbox.WriteVariant(dir, id, variants[id]); err != nil {
			return nil, err
		}
	}
	res, err := r.Run(ctx, sandbox.Batch{Dir: dir, IDs: ids})
	if err != nil {
		return nil, err
	}
	out := make(map[string]sandbox.VariantResult, len(res))
	for _, v := range res {
		out[v.ID] = v
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			return nil, fmt.Errorf("runner returned no result for variant %s", id)
		}
	}
	return out, nil
}

// checkBaseline verifies that every baseline run is green with the same tests.
func checkBaseline(ids []string, base map[string]sandbox.VariantResult) ([]string, error) {
	keys := base[ids[0]].Keys()
	for _, id := range ids {
		v := base[id]
		if v.BuildError != "" {
			return nil, fmt.Errorf("%w: build error: %s", ErrBaselineRed, v.BuildError)
		}
		var failed []string
		for _, t := range v.Tests {
			if t.Status != sandbox.StatusPass {
				failed = append(failed, t.Key())
			}
		}
		if len(failed) > 0 {
			sort.Strings(failed)
			return nil, fmt.Errorf("%w: failing tests: %s", ErrBaselineRed, strings.Join(failed, ", "))
		}
		if !slices.Equal(v.Keys(), keys) {
			return nil, fmt.Errorf("%w: test set differs between runs", ErrBaselineRed)
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no tests found", ErrBaselineRed)
	}
	return keys, nil
}

// evaluate runs every candidate mutation on its own and keeps the ones that
// make at least one test fail without a build error or timeout.
func evaluate(ctx context.Context, files, goFiles map[string][]byte, r sandbox.Runner,
	workDir string) ([]Candidate, map[string]sandbox.VariantResult, Stats, error) {
	ms, err := mutate.Generate(goFiles)
	if err != nil {
		return nil, nil, Stats{}, err
	}
	stats := Stats{Candidates: len(ms)}
	if len(ms) == 0 {
		return nil, nil, stats, nil
	}
	ids := make([]string, 0, len(ms))
	vars := make(map[string]map[string][]byte, len(ms))
	for _, m := range ms {
		mf, err := mutate.Apply(files, []mutate.Mutation{m})
		if err != nil {
			return nil, nil, stats, err
		}
		ids = append(ids, m.ID)
		vars[m.ID] = mf
	}
	res, err := runBatch(ctx, r, workDir, "cands", ids, vars)
	if err != nil {
		return nil, nil, stats, err
	}
	var cands []Candidate
	kept := map[string]sandbox.VariantResult{}
	for _, m := range ms {
		v := res[m.ID]
		if v.BuildError != "" {
			stats.BuildErrors++
			continue
		}
		c := Candidate{M: m}
		for _, t := range v.Tests {
			if t.Status == sandbox.StatusPass {
				continue
			}
			c.Failing = append(c.Failing, t.Key())
			c.Timeout = c.Timeout || t.Status == sandbox.StatusTimeout
			c.Panicked = c.Panicked || t.Panicked
		}
		switch {
		case c.Timeout:
		case len(c.Failing) == 0:
			stats.Survived++
		default:
			sort.Strings(c.Failing)
			stats.Killed++
			cands = append(cands, c)
			kept[m.ID] = v
		}
	}
	return cands, kept, stats, nil
}

// triple is one triple under verification, sorted by (file, offset) so bit i
// of a variant mask refers to final mutation i.
type triple struct {
	cands [3]Candidate
	ms    [3]mutate.Mutation // renamed m1..m3
}

func newTriple(c [3]Candidate) triple {
	sort.Slice(c[:], func(a, b int) bool {
		if c[a].M.File != c[b].M.File {
			return c[a].M.File < c[b].M.File
		}
		return c[a].M.Start < c[b].M.Start
	})
	t := triple{cands: c}
	for i := range c {
		t.ms[i] = c[i].M
		t.ms[i].ID = "m" + strconv.Itoa(i+1)
	}
	return t
}

func (t triple) subset(mask int) []mutate.Mutation {
	var sel []mutate.Mutation
	for i := 0; i < 3; i++ {
		if mask&(1<<i) != 0 {
			sel = append(sel, t.ms[i])
		}
	}
	return sel
}

var combinedMasks = []int{3, 5, 6, 7}

// Prepare chooses three independent mutations of the project.
// files is the whole project, goFiles the non-test .go files.
func Prepare(ctx context.Context, files map[string][]byte, goFiles map[string][]byte,
	r sandbox.Runner, workDir string, opt Options, onProgress func(Progress)) (*Plan, error) {
	if onProgress == nil {
		onProgress = func(Progress) {}
	}
	if opt.BaselineRuns < 1 {
		opt.BaselineRuns = 1
	}
	if opt.TriplesPerBatch < 1 {
		opt.TriplesPerBatch = 1
	}

	// 1. Baseline.
	ids := make([]string, opt.BaselineRuns)
	vars := map[string]map[string][]byte{}
	for i := range ids {
		ids[i] = "b" + strconv.Itoa(i+1)
		vars[ids[i]] = files
	}
	base, err := runBatch(ctx, r, workDir, "baseline", ids, vars)
	if err != nil {
		return nil, err
	}
	onProgress(Progress{"baseline", 1, 1})
	baseKeys, err := checkBaseline(ids, base)
	if err != nil {
		return nil, err
	}
	baseline := base[ids[0]]

	// 2. Single-mutation candidates.
	cands, candRes, stats, err := evaluate(ctx, files, goFiles, r, workDir)
	if err != nil {
		return nil, err
	}
	onProgress(Progress{"candidates", 1, 1})

	// 3. Rank and enumerate triples.
	sort.SliceStable(cands, func(i, j int) bool { return Score(cands[i]) > Score(cands[j]) })
	ranked := rankTriples(cands)
	if opt.MaxTriples > 0 && len(ranked) > opt.MaxTriples {
		ranked = ranked[:opt.MaxTriples]
	}

	// 4. Verify triples batch by batch.
	for start := 0; start < len(ranked); start += opt.TriplesPerBatch {
		end := min(start+opt.TriplesPerBatch, len(ranked))
		jobs := map[int]triple{}
		ids = ids[:0]
		vars = map[string]map[string][]byte{}
		for k := start; k < end; k++ {
			stats.TriplesChecked++
			ix := ranked[k]
			t := newTriple([3]Candidate{cands[ix[0]], cands[ix[1]], cands[ix[2]]})
			tv := map[string]map[string][]byte{}
			overlap := false
			for _, mask := range combinedMasks {
				mf, err := mutate.Apply(files, t.subset(mask))
				if errors.Is(err, mutate.ErrOverlap) {
					overlap = true
					break
				}
				if err != nil {
					return nil, err
				}
				tv[variantID(k, mask)] = mf
			}
			if overlap {
				continue
			}
			jobs[k] = t
			for _, mask := range combinedMasks {
				id := variantID(k, mask)
				ids = append(ids, id)
				vars[id] = tv[id]
			}
		}
		if len(jobs) > 0 {
			res, err := runBatch(ctx, r, workDir, "triples", ids, vars)
			if err != nil {
				return nil, err
			}
			for k := start; k < end; k++ {
				t, ok := jobs[k]
				if !ok {
					continue
				}
				var rs [8]sandbox.VariantResult
				rs[0] = baseline
				for i := 0; i < 3; i++ {
					rs[1<<i] = candRes[t.cands[i].M.ID]
				}
				for _, mask := range combinedMasks {
					rs[mask] = res[variantID(k, mask)]
				}
				killing, ok := Independent(rs, baseKeys)
				if !ok {
					continue
				}
				mutated, err := mutate.Apply(files, t.ms[:])
				if err != nil {
					return nil, err
				}
				onProgress(Progress{"triples", stats.TriplesChecked, len(ranked)})
				onProgress(Progress{"done", 1, 1})
				return &Plan{
					Mutations:     t.ms[:],
					KillingTests:  killing[:],
					BaselineTests: baseKeys,
					Mutated:       mutated,
					Stats:         stats,
				}, nil
			}
		}
		onProgress(Progress{"triples", stats.TriplesChecked, len(ranked)})
	}
	return nil, ErrNoTriple
}

func variantID(k, mask int) string { return fmt.Sprintf("t%d_%d", k, mask) }

package selector

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"sort"
	"strconv"

	"lcme/internal/mutate"
	"lcme/internal/sandbox"
)

// Options tunes Prepare.
type Options struct {
	BaselineRuns, MaxTriples, TriplesPerBatch, MaxCandidates int
	// Seed zero preserves deterministic selection for tests.
	Seed int64
}

// DefaultOptions are the values used in production.
var DefaultOptions = Options{BaselineRuns: 3, MaxTriples: 30, TriplesPerBatch: 5, MaxCandidates: 64}

// Progress is reported after every sandbox batch.
type Progress struct {
	Stage       string // "baseline" | "candidates" | "triples" | "done"
	Done, Total int
}

// Stats summarises the selection process.
type Stats struct{ Candidates, SelectedCandidates, Killed, Survived, BuildErrors, TriplesChecked int }

// Plan is the prepared exercise.
type Plan struct {
	Mutations     []mutate.Mutation // IDs m1..m3 in (file, offset) order
	KillingTests  [][]string        // [i] fails exactly when mutation i is applied
	BaselineTests []string
	Mutated       map[string][]byte `json:"-"`
	Stats         Stats
}

var (
	ErrBaselineRed    = errors.New("baseline is not green")
	ErrNoTriple       = errors.New("no independent triple")
	ErrInfrastructure = errors.New("sandbox infrastructure failure")
)

// runBatch writes the variants, runs them and removes the batch directory.
// Results are returned keyed by variant ID.
func runBatch(ctx context.Context, r sandbox.Runner, workDir, name string,
	ids []string, variant func(int) (map[string][]byte, error)) (map[string]sandbox.VariantResult, error) {
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
	defer func() { os.RemoveAll(dir) }()
	if err := os.Chmod(dir, 0o755); err != nil {
		return nil, err
	}
	out := make(map[string]sandbox.VariantResult, len(ids))
	var staged []string
	var batchBytes int64
	flush := func() error {
		if len(staged) == 0 {
			return nil
		}
		res, err := r.Run(ctx, sandbox.Batch{Dir: dir, IDs: staged})
		if err != nil {
			return err
		}
		for _, v := range res {
			if v.InfrastructureError != "" {
				return ErrInfrastructure
			}
			out[v.ID] = v
		}
		for _, id := range staged {
			if _, ok := out[id]; !ok {
				return fmt.Errorf("runner returned no result for variant %s", id)
			}
		}
		return nil
	}
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, err := variant(i)
		if err != nil {
			return nil, err
		}
		var size int64
		for _, src := range files {
			size += int64(len(src))
		}
		if size > maxCandidateBatchBytes {
			return nil, fmt.Errorf("variant %s exceeds %d-byte batch limit", id, maxCandidateBatchBytes)
		}
		if len(staged) >= maxCandidateBatchVariants || batchBytes+size > maxCandidateBatchBytes {
			if err := flush(); err != nil {
				return nil, err
			}
			if err := os.RemoveAll(dir); err != nil {
				return nil, err
			}
			dir, err = os.MkdirTemp(workDir, name+"-")
			if err != nil {
				return nil, err
			}
			if err := os.Chmod(dir, 0o755); err != nil {
				return nil, err
			}
			staged, batchBytes = nil, 0
		}
		if err := sandbox.WriteVariant(dir, id, files); err != nil {
			return nil, err
		}
		staged = append(staged, id)
		batchBytes += size
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

// checkBaseline verifies that every baseline run is green with the same tests.
func checkBaseline(ids []string, base map[string]sandbox.VariantResult) ([]string, error) {
	keys := base[ids[0]].Keys()
	for _, id := range ids {
		v := base[id]
		if v.InfrastructureError != "" {
			return nil, ErrInfrastructure
		}
		if v.BuildError != "" {
			return nil, baselineError("build", nil)
		}
		var failed []string
		for _, t := range v.Tests {
			if t.Status != sandbox.StatusPass {
				failed = append(failed, t.Key())
			}
		}
		if len(failed) > 0 {
			sort.Strings(failed)
			return nil, baselineError("tests", failed)
		}
		if !slices.Equal(v.Keys(), keys) {
			return nil, baselineError("test_set", nil)
		}
	}
	if len(keys) == 0 {
		return nil, baselineError("no_tests", nil)
	}
	return keys, nil
}

const (
	maxCandidateBatchVariants = 16
	maxCandidateBatchBytes    = 32 << 20
)

// evaluate runs every candidate on its own and retains the highest-scoring
// mutations that fail a test without a build error or timeout.
func evaluate(ctx context.Context, files, goFiles map[string][]byte, r sandbox.Runner,
	workDir string, baseKeys []string, onProgress func(Progress)) ([]Candidate, map[string]sandbox.VariantResult, Stats, error) {
	return evaluateOptions(ctx, files, goFiles, r, workDir, baseKeys, DefaultOptions, onProgress)
}

func evaluateOptions(ctx context.Context, files, goFiles map[string][]byte, r sandbox.Runner,
	workDir string, baseKeys []string, opt Options, onProgress func(Progress)) ([]Candidate, map[string]sandbox.VariantResult, Stats, error) {
	ms, err := mutate.Generate(goFiles)
	if err != nil {
		return nil, nil, Stats{}, err
	}
	stats := Stats{Candidates: len(ms)}
	ms = selectCandidates(ms, opt)
	stats.SelectedCandidates = len(ms)
	if len(ms) == 0 {
		onProgress(Progress{"candidates", 1, 1})
		return nil, nil, stats, nil
	}
	var projectBytes int64
	for _, src := range files {
		projectBytes += int64(len(src))
	}
	cands := make([]Candidate, 0, len(ms))
	kept := map[string]sandbox.VariantResult{}
	for start := 0; start < len(ms); {
		ids := make([]string, 0, maxCandidateBatchVariants)
		var batchBytes int64
		for _, m := range ms[start:min(start+maxCandidateBatchVariants, len(ms))] {
			variantBytes := projectBytes + int64(len(m.Repl)-(m.End-m.Start))
			if variantBytes > maxCandidateBatchBytes {
				return nil, nil, stats, fmt.Errorf("candidate %s exceeds %d-byte batch limit", m.ID, maxCandidateBatchBytes)
			}
			if batchBytes+variantBytes > maxCandidateBatchBytes {
				break
			}
			ids = append(ids, m.ID)
			batchBytes += variantBytes
		}
		batch := ms[start : start+len(ids)]
		res, err := runBatch(ctx, r, workDir, "cands", ids, func(i int) (map[string][]byte, error) {
			return mutate.Apply(files, batch[i:i+1])
		})
		if err != nil {
			return nil, nil, stats, err
		}
		for _, m := range batch {
			v := res[m.ID]
			if c, ok := evaluateCandidate(m, v, baseKeys, &stats); ok {
				cands = append(cands, c)
				kept[m.ID] = v
			}
		}
		sort.SliceStable(cands, func(i, j int) bool { return Score(cands[i]) > Score(cands[j]) })
		start += len(batch)
		onProgress(Progress{"candidates", start, len(ms)})
	}
	return cands, kept, stats, nil
}

// selectCandidates allocates the sandbox budget across functions. Large
// functions cannot consume it before later functions get a candidate.
func selectCandidates(ms []mutate.Mutation, opt Options) []mutate.Mutation {
	limit := opt.MaxCandidates
	if limit < 1 {
		limit = DefaultOptions.MaxCandidates
	}
	if opt.Seed == 0 && len(ms) <= limit {
		return ms
	}
	groups := map[string][]mutate.Mutation{}
	var names []string
	for _, m := range ms {
		key := m.File + "\x00" + m.Func
		if _, ok := groups[key]; !ok {
			names = append(names, key)
		}
		groups[key] = append(groups[key], m)
	}
	var rng *rand.Rand
	if opt.Seed != 0 {
		rng = rand.New(rand.NewSource(opt.Seed))
		rng.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
	}
	for _, name := range names {
		group := groups[name]
		if rng != nil {
			rng.Shuffle(len(group), func(i, j int) { group[i], group[j] = group[j], group[i] })
		}
		sort.SliceStable(group, func(i, j int) bool { return Score(Candidate{M: group[i]}) > Score(Candidate{M: group[j]}) })
	}
	selected := make([]mutate.Mutation, 0, min(limit, len(ms)))
	for round := 0; len(selected) < min(limit, len(ms)); round++ {
		for _, name := range names {
			if group := groups[name]; round < len(group) {
				selected = append(selected, group[round])
			}
			if len(selected) == limit {
				break
			}
		}
	}
	return selected
}

func evaluateCandidate(m mutate.Mutation, v sandbox.VariantResult, baseKeys []string, stats *Stats) (Candidate, bool) {
	// A different test set (e.g. a synthetic "(init)" result) means the
	// variant could not run as a normal one.
	if v.BuildError != "" || !slices.Equal(v.Keys(), baseKeys) {
		stats.BuildErrors++
		return Candidate{}, false
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
		return Candidate{}, false
	case len(c.Failing) == 0:
		stats.Survived++
		return Candidate{}, false
	default:
		sort.Strings(c.Failing)
		stats.Killed++
		return c, true
	}
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
	for i := range ids {
		ids[i] = "b" + strconv.Itoa(i+1)
	}
	base, err := runBatch(ctx, r, workDir, "baseline", ids, func(int) (map[string][]byte, error) {
		return files, nil
	})
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
	cands, candRes, stats, err := evaluateOptions(ctx, files, goFiles, r, workDir, baseKeys, opt, onProgress)
	if err != nil {
		return nil, err
	}
	// 3. Rank and enumerate triples.
	ranked := rankTriples(cands)
	if opt.MaxTriples > 0 && len(ranked) > opt.MaxTriples {
		ranked = ranked[:opt.MaxTriples]
	}

	// 4. Verify triples batch by batch.
	for start := 0; start < len(ranked); start += opt.TriplesPerBatch {
		end := min(start+opt.TriplesPerBatch, len(ranked))
		jobs := map[int]triple{}
		ids = ids[:0]
		var variants [][]mutate.Mutation
		for k := start; k < end; k++ {
			stats.TriplesChecked++
			ix := ranked[k]
			t := newTriple([3]Candidate{cands[ix[0]], cands[ix[1]], cands[ix[2]]})
			overlap := false
			for i := 1; i < len(t.ms); i++ {
				if a, b := t.ms[i-1], t.ms[i]; a.File == b.File && (b.Start < a.End || b.Start == a.Start) {
					overlap = true
				}
			}
			if overlap {
				continue
			}
			jobs[k] = t
			for _, mask := range combinedMasks {
				id := variantID(k, mask)
				ids = append(ids, id)
				variants = append(variants, t.subset(mask))
			}
		}
		if len(jobs) > 0 {
			res, err := runBatch(ctx, r, workDir, "triples", ids, func(i int) (map[string][]byte, error) {
				return mutate.Apply(files, variants[i])
			})
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

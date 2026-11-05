package selector

import (
	"sort"
	"strings"
)

// tripleIdx is a triple of indexes into the ranked candidate slice.
type tripleIdx [3]int

// rankTriples returns triples drawn from the best candidates, ordered by
// decreasing total score. Triples need three distinct functions and pairwise
// different failing-test sets. cands must already be sorted by Score.
func rankTriples(cands []Candidate) []tripleIdx {
	n := len(cands)
	failKey := make([]string, n)
	for i := 0; i < n; i++ {
		failKey[i] = strings.Join(cands[i].Failing, "\x00")
	}
	compatible := func(a, b int) bool {
		return cands[a].M.Func != cands[b].M.Func && failKey[a] != failKey[b]
	}
	var out []tripleIdx
	for a := 0; a < n; a++ {
		for b := a + 1; b < n; b++ {
			if !compatible(a, b) {
				continue
			}
			for c := b + 1; c < n; c++ {
				if compatible(a, c) && compatible(b, c) {
					out = append(out, tripleIdx{a, b, c})
				}
			}
		}
	}
	sum := func(t tripleIdx) int {
		return Score(cands[t[0]]) + Score(cands[t[1]]) + Score(cands[t[2]])
	}
	sort.SliceStable(out, func(i, j int) bool { return sum(out[i]) > sum(out[j]) })
	return out
}

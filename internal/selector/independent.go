// Package selector picks three mutations that fail independent tests and
// verifies the choice against all 8 combinations of applying them.
package selector

import (
	"lcme/internal/mutate"
	"lcme/internal/sandbox"
)

// Candidate is a single mutation together with what happened when it was
// applied on its own.
type Candidate struct {
	M        mutate.Mutation
	Failing  []string
	Panicked bool
	Timeout  bool
}

// Score ranks candidates: higher is a better exercise.
func Score(c Candidate) int {
	s := 0
	switch c.M.Kind {
	case mutate.Boundary, mutate.Negate, mutate.Logical:
		s = 10
	case mutate.IncDec, mutate.Arith:
		s = 6
	case mutate.Constant:
		s = 2
	}
	if c.Panicked {
		s -= 8
	}
	if c.M.Func != "main" {
		s += 2
	}
	return s
}

// Independent checks the 8 variants r, where r[mask] has mutation i applied
// iff mask&(1<<i) != 0. killing[i] holds the tests that fail exactly when
// mutation i is present. ok is true when all three lists are non-empty and
// no variant failed to build.
func Independent(r [8]sandbox.VariantResult, tests []string) (killing [3][]string, ok bool) {
	for _, v := range r {
		if v.BuildError != "" {
			return killing, false
		}
	}
	for _, t := range tests {
		for i := 0; i < 3; i++ {
			good := true
			for mask := 0; mask < 8; mask++ {
				if r[mask].Passed(t) != (mask&(1<<i) == 0) {
					good = false
					break
				}
			}
			if good {
				killing[i] = append(killing[i], t)
			}
		}
	}
	ok = len(killing[0]) > 0 && len(killing[1]) > 0 && len(killing[2]) > 0
	return killing, ok
}

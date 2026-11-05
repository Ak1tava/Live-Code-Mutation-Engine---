package selector

import "strings"

// BaselineError provides bounded diagnostics safe to expose to the auditor.
// Kind is build, tests, test_set, or no_tests. Raw runner output is excluded.
type BaselineError struct {
	Kind         string
	FailingTests []string
}

func (e *BaselineError) Unwrap() error { return ErrBaselineRed }

func (e *BaselineError) Error() string {
	message := ErrBaselineRed.Error() + ": " + e.Kind
	if len(e.FailingTests) > 0 {
		message += ": " + strings.Join(e.FailingTests, ", ")
	}
	return message
}

func baselineError(kind string, failed []string) error {
	e := &BaselineError{Kind: kind}
	for _, key := range failed {
		if len(e.FailingTests) == 8 {
			break
		}
		if len(key) > 160 {
			continue
		}
		safe := true
		for _, ch := range key {
			if !(ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || strings.ContainsRune("_./-:", ch)) {
				safe = false
				break
			}
		}
		if safe {
			e.FailingTests = append(e.FailingTests, key)
		}
	}
	return e
}

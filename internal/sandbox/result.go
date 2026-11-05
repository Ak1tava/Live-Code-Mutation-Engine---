// Package sandbox defines the protocol between the server and the test runner
// and the Runner implementations that execute variants.
package sandbox

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Status is the outcome of a single test.
type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusTimeout Status = "timeout"
)

const (
	// TestTimeout is the per-test timeout passed as -test.timeout.
	TestTimeout = 2 * time.Second
	// OutputCap is the maximum number of bytes of test output kept.
	OutputCap = 4096
)

// TestResult is the outcome of one test run in its own process.
type TestResult struct {
	Pkg      string `json:"pkg"`
	Name     string `json:"name"`
	Status   Status `json:"status"`
	Panicked bool   `json:"panicked"`
	Output   string `json:"output"`
}

// Key identifies a test, e.g. ".:TestMax".
func (t TestResult) Key() string { return t.Pkg + ":" + t.Name }

// VariantResult is the outcome of running all tests of one variant.
type VariantResult struct {
	ID         string       `json:"id"`
	BuildError string       `json:"buildError"`
	Tests      []TestResult `json:"tests"`
	DurationMs int64        `json:"durationMs"`
}

// Passed reports whether the test with the given key ran and passed.
func (v VariantResult) Passed(key string) bool {
	for _, t := range v.Tests {
		if t.Key() == key {
			return t.Status == StatusPass
		}
	}
	return false
}

// AllPassed reports whether every key passed.
func (v VariantResult) AllPassed(keys []string) bool {
	for _, k := range keys {
		if !v.Passed(k) {
			return false
		}
	}
	return true
}

// Keys returns the sorted keys of all tests.
func (v VariantResult) Keys() []string {
	keys := make([]string, 0, len(v.Tests))
	for _, t := range v.Tests {
		keys = append(keys, t.Key())
	}
	sort.Strings(keys)
	return keys
}

// ParseResults reads one JSON VariantResult per line from r, failing if more
// than limit bytes are available.
func ParseResults(r io.Reader, limit int64) ([]VariantResult, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("runner output exceeds %d bytes", limit)
	}
	var out []VariantResult
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), int(limit)+1)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var v VariantResult
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			return nil, fmt.Errorf("parse runner output: %w", err)
		}
		out = append(out, v)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

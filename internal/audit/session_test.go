package audit

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"lcme/internal/hints"
	"lcme/internal/mutate"
	"lcme/internal/project"
	"lcme/internal/sandbox"
	"lcme/internal/selector"
)

var t0 = time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

func testPlan() *selector.Plan {
	return &selector.Plan{
		Mutations: []mutate.Mutation{
			{ID: "m1", File: "textkit.go", Line: 3, Kind: mutate.Boundary, Orig: "<", Repl: "<="},
			{ID: "m2", File: "textkit.go", Line: 7, Kind: mutate.Negate, Orig: "==", Repl: "!="},
			{ID: "m3", File: "textkit.go", Line: 9, Kind: mutate.Arith, Orig: "+", Repl: "-"},
		},
		KillingTests:  [][]string{{".:TestA"}, {".:TestB"}, {".:TestC"}},
		BaselineTests: []string{".:TestA", ".:TestB", ".:TestC"},
		Mutated: map[string][]byte{
			"textkit.go":      []byte("package textkit\n"),
			"textkit_test.go": []byte("package textkit\n"),
			"go.mod":          []byte("module x\n"),
		},
	}
}

func readySession(t *testing.T) *Session {
	t.Helper()
	s := New(project.Source{Example: "textkit"}, t0)
	s.SetReady(&project.Project{}, testPlan(), []hints.Hint{{MutationID: "m1"}})
	return s
}

func TestNewTokens(t *testing.T) {
	s := New(project.Source{}, t0)
	if len(s.ID) != 32 || len(s.AuditorToken) != 32 || len(s.StudentToken) != 32 {
		t.Fatalf("want 32 hex chars: %q %q %q", s.ID, s.AuditorToken, s.StudentToken)
	}
	if s.AuditorToken == s.StudentToken || s.State != StatePreparing {
		t.Fatalf("bad new session: %+v", s)
	}
}

func TestSetReadySplitsAndCopies(t *testing.T) {
	s := readySession(t)
	if s.State != StateReady {
		t.Fatalf("state = %s", s.State)
	}
	if _, ok := s.Files["textkit.go"]; !ok || len(s.Files) != 1 {
		t.Errorf("Files = %v", s.Files)
	}
	if _, ok := s.Tests["textkit_test.go"]; !ok || len(s.Tests) != 2 {
		t.Errorf("Tests = %v", s.Tests)
	}
	if _, ok := s.Tests["go.mod"]; !ok {
		t.Errorf("go.mod must be read-only")
	}
	s.Files["textkit.go"][0] = 'X'
	if s.Mutated["textkit.go"][0] == 'X' {
		t.Fatal("editing Files changed Mutated")
	}
	s.Tests["go.mod"][0] = 'X'
	if s.Mutated["go.mod"][0] == 'X' {
		t.Fatal("editing Tests changed Mutated")
	}
}

func TestStartSetsDeadline(t *testing.T) {
	s := readySession(t)
	if err := s.Start(t0); err != nil {
		t.Fatal(err)
	}
	if s.State != StateRunning || !s.Deadline.Equal(t0.Add(10*time.Minute)) || !s.StartedAt.Equal(t0) {
		t.Fatalf("state=%s deadline=%v", s.State, s.Deadline)
	}
	if err := s.Start(t0.Add(time.Second)); !errors.Is(err, ErrBadState) {
		t.Fatalf("second Start = %v", err)
	}
}

func TestStartRequiresReady(t *testing.T) {
	s := New(project.Source{}, t0)
	if err := s.Start(t0); !errors.Is(err, ErrBadState) {
		t.Fatalf("Start on preparing = %v", err)
	}
	s.Fail("boom")
	if s.State != StateFailed || s.Error != "boom" {
		t.Fatalf("Fail: %+v", s)
	}
}

func TestUpdateFileRules(t *testing.T) {
	s := readySession(t)
	if err := s.UpdateFile("textkit.go", []byte("x"), t0); !errors.Is(err, ErrBadState) {
		t.Fatalf("before Start = %v", err)
	}
	if err := s.Start(t0); err != nil {
		t.Fatal(err)
	}
	now := t0.Add(time.Minute)
	if err := s.UpdateFile("textkit_test.go", []byte("x"), now); !errors.Is(err, ErrReadOnly) {
		t.Errorf("test file = %v", err)
	}
	if err := s.UpdateFile("nope.go", []byte("x"), now); !errors.Is(err, ErrUnknownFile) {
		t.Errorf("unknown = %v", err)
	}
	if err := s.UpdateFile("textkit.go", make([]byte, 300<<10), now); !errors.Is(err, ErrTooBig) {
		t.Errorf("big = %v", err)
	}
	if err := s.UpdateFile("textkit.go", make([]byte, MaxFileSize), now); err != nil {
		t.Errorf("exactly max = %v", err)
	}
	if err := s.UpdateFile("textkit.go", []byte("new"), t0.Add(10*time.Minute+time.Nanosecond)); !errors.Is(err, ErrExpired) {
		t.Errorf("expired = %v", err)
	}
	if err := s.UpdateFile("textkit.go", []byte("new"), t0.Add(10*time.Minute)); err != nil {
		t.Errorf("at deadline = %v", err)
	}
	if !bytes.Equal(s.Files["textkit.go"], []byte("new")) {
		t.Errorf("Files not updated: %q", s.Files["textkit.go"])
	}
}

func TestRunFilesTestsOverride(t *testing.T) {
	s := readySession(t)
	s.Files["textkit_test.go"] = []byte("student version")
	got := s.RunFiles()
	if string(got["textkit_test.go"]) != "package textkit\n" {
		t.Fatalf("tests must override: %q", got["textkit_test.go"])
	}
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestExpiredAndFinish(t *testing.T) {
	s := readySession(t)
	if s.Expired(t0.Add(time.Hour)) {
		t.Error("ready session cannot expire")
	}
	_ = s.Start(t0)
	if s.Expired(t0.Add(10 * time.Minute)) {
		t.Error("not expired at deadline")
	}
	if !s.Expired(t0.Add(10*time.Minute + time.Second)) {
		t.Error("expired after deadline")
	}
	first := t0.Add(11 * time.Minute)
	s.Finish(first)
	s.Finish(first.Add(time.Hour))
	if s.State != StateFinished || !s.FinishedAt.Equal(first) {
		t.Errorf("Finish: %s %v", s.State, s.FinishedAt)
	}
	if s.Expired(first.Add(time.Hour)) {
		t.Error("finished session is not expired")
	}
}

func TestFlagsAndReview(t *testing.T) {
	s := readySession(t)
	_ = s.Start(t0)
	f, err := s.AddFlag("textkit.go", 3, "here", t0.Add(time.Second))
	if err != nil || f.ID == "" || f.Accepted != nil {
		t.Fatalf("AddFlag: %+v %v", f, err)
	}
	if _, err := s.AddFlag("textkit.go", 3, "late", t0.Add(11*time.Minute)); !errors.Is(err, ErrExpired) {
		t.Errorf("late flag = %v", err)
	}
	if err := s.ReviewFlag("zzz", "m1", true); !errors.Is(err, ErrUnknownFlag) {
		t.Errorf("unknown flag = %v", err)
	}
	if err := s.ReviewFlag(f.ID, "m9", true); !errors.Is(err, ErrUnknownMutation) {
		t.Errorf("unknown mutation = %v", err)
	}
	if err := s.ReviewFlag(f.ID, "m1", false); err != nil {
		t.Fatal(err)
	}
	if s.Flags[0].MutationID != "m1" || s.Flags[0].Accepted == nil || *s.Flags[0].Accepted {
		t.Errorf("flag after review: %+v", s.Flags[0])
	}
}

func TestAddEventAndRun(t *testing.T) {
	s := readySession(t)
	_ = s.AddEvent("paste", 5, t0)
	s.AddRun(sandbox.VariantResult{ID: "x"}, true, t0)
	if len(s.Events) != 1 || len(s.Runs) != 1 || !s.Runs[0].Final || s.Events[0].Size != 5 {
		t.Fatalf("%+v %+v", s.Events, s.Runs)
	}
	if strings.TrimSpace(s.Events[0].Kind) != "paste" {
		t.Fatal("kind")
	}
}

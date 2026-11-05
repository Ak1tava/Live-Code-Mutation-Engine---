package audit

import (
	"testing"
	"time"

	"lcme/internal/sandbox"
)

func vr(pass map[string]bool) sandbox.VariantResult {
	v := sandbox.VariantResult{ID: "student"}
	for _, n := range []string{"TestA", "TestB", "TestC"} {
		st := sandbox.StatusFail
		if pass[n] {
			st = sandbox.StatusPass
		}
		v.Tests = append(v.Tests, sandbox.TestResult{Pkg: ".", Name: n, Status: st})
	}
	return v
}

func runningSession(t *testing.T) *Session {
	s := readySession(t)
	if err := s.Start(t0); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestReportFixedByTestsNotText(t *testing.T) {
	s := runningSession(t)
	s.Files["textkit.go"] = []byte("package textkit\n// a different solution\n")
	s.AddRun(vr(map[string]bool{"TestA": true, "TestB": true}), true, t0.Add(5*time.Minute))
	s.Finish(t0.Add(6 * time.Minute))
	r := BuildReport(s)
	got := []bool{r.Mutations[0].Fixed, r.Mutations[1].Fixed, r.Mutations[2].Fixed}
	if got[0] != true || got[1] != true || got[2] != false {
		t.Fatalf("Fixed = %v", got)
	}
	if r.Restored {
		t.Error("Restored must be false")
	}
	if r.DurationSec != 360 {
		t.Errorf("DurationSec = %d", r.DurationSec)
	}
}

func TestReportRestored(t *testing.T) {
	s := runningSession(t)
	all := map[string]bool{"TestA": true, "TestB": true, "TestC": true}
	s.AddRun(vr(all), true, t0.Add(time.Minute))
	if r := BuildReport(s); !r.Restored {
		t.Error("want Restored")
	}
	bad := vr(all)
	bad.BuildError = "oops"
	s.AddRun(bad, true, t0.Add(2*time.Minute))
	if r := BuildReport(s); r.Restored {
		t.Error("build error: not restored")
	}
}

func TestReportNonFinalRunDoesNotFix(t *testing.T) {
	s := runningSession(t)
	s.AddRun(vr(map[string]bool{"TestA": true}), false, t0.Add(time.Minute))
	r := BuildReport(s)
	if r.Mutations[0].Fixed {
		t.Error("Fixed must use only final runs")
	}
	if r.Mutations[0].FixedAfterSec != 60 {
		t.Errorf("FixedAfterSec = %d, want 60 (any run counts)", r.Mutations[0].FixedAfterSec)
	}
}

func TestReportDetectedExplained(t *testing.T) {
	s := runningSession(t)
	f1, _ := s.AddFlag("textkit.go", 7, "a", t0.Add(time.Second))
	f2, _ := s.AddFlag("textkit.go", 3, "b", t0.Add(2*time.Second))
	if err := s.ReviewFlag(f1.ID, "m2", true); err != nil {
		t.Fatal(err)
	}
	if err := s.ReviewFlag(f2.ID, "m1", false); err != nil {
		t.Fatal(err)
	}
	r := BuildReport(s)
	m := r.Mutations
	if !m[1].Detected || !m[1].Explained {
		t.Errorf("m2: %+v", m[1])
	}
	if !m[0].Detected || m[0].Explained {
		t.Errorf("m1: %+v", m[0])
	}
	if m[2].Detected || m[2].Explained {
		t.Errorf("m3: %+v", m[2])
	}
}

func TestReportFixedAfterSec(t *testing.T) {
	s := runningSession(t)
	s.AddRun(vr(nil), false, t0.Add(60*time.Second))
	s.AddRun(vr(map[string]bool{"TestA": true}), false, t0.Add(120*time.Second))
	r := BuildReport(s)
	if r.Mutations[0].FixedAfterSec != 120 {
		t.Errorf("m1 = %d", r.Mutations[0].FixedAfterSec)
	}
	if r.Mutations[1].FixedAfterSec != -1 || r.Mutations[2].FixedAfterSec != -1 {
		t.Errorf("others: %d %d", r.Mutations[1].FixedAfterSec, r.Mutations[2].FixedAfterSec)
	}
}

func TestReportSignals(t *testing.T) {
	s := runningSession(t)
	_ = s.AddEvent("hidden", 0, t0)
	_ = s.AddEvent("visible", 0, t0)
	_ = s.AddEvent("hidden", 0, t0)
	_ = s.AddEvent("paste", 120, t0)
	_ = s.AddEvent("paste", 30, t0)
	r := BuildReport(s)
	if r.HiddenCount != 2 || r.PasteCount != 2 || r.PasteChars != 150 {
		t.Fatalf("%+v", r)
	}
}

func TestReportWithoutPlan(t *testing.T) {
	r := BuildReport(New(sourceZero(), t0))
	if r.Mutations == nil || len(r.Mutations) != 0 {
		t.Fatalf("Mutations = %#v", r.Mutations)
	}
}

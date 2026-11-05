package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"lcme/internal/project"
	"lcme/internal/sandbox"
)

func sourceZero() project.Source { return project.Source{} }

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := readySession(t)
	if err := st.Create(s); err != nil {
		t.Fatal(err)
	}
	err = st.Update(s.ID, func(x *Session) error {
		if err := x.Start(t0); err != nil {
			return err
		}
		if err := x.UpdateFile("textkit.go", []byte("package textkit\n// edited\n"), t0.Add(time.Second)); err != nil {
			return err
		}
		f, err := x.AddFlag("textkit.go", 3, "note", t0.Add(2*time.Second))
		if err != nil {
			return err
		}
		if err := x.ReviewFlag(f.ID, "m1", true); err != nil {
			return err
		}
		_, _ = x.AddFlag("textkit.go", 4, "unreviewed", t0.Add(3*time.Second))
		_ = x.AddEvent("paste", 7, t0.Add(4*time.Second))
		x.FinishRequestedAt = t0.Add(5 * time.Second)
		x.AddRun(vr(map[string]bool{"TestA": true}), true, t0.Add(5*time.Second))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	st2, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	var want, got *Session
	st.View(s.ID, func(x *Session) { want = x.clone() })
	st2.View(s.ID, func(x *Session) { got = x.clone() })
	if got == nil {
		t.Fatal("session not loaded")
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip differs:\nwant %+v\ngot  %+v", want, got)
	}
	if !bytes.Equal(got.Files["textkit.go"], []byte("package textkit\n// edited\n")) {
		t.Errorf("Files lost: %q", got.Files["textkit.go"])
	}
	if got.Plan == nil || !bytes.Equal(got.Plan.Mutated["textkit.go"], []byte("package textkit\n")) {
		t.Errorf("Plan.Mutated lost: %+v", got.Plan)
	}
	if got.Flags[0].Accepted == nil || !*got.Flags[0].Accepted || got.Flags[1].Accepted != nil {
		t.Errorf("Accepted nil/true not preserved: %+v", got.Flags)
	}
	if id, role, ok := st2.ByToken(s.AuditorToken); !ok || id != s.ID || role != RoleAuditor {
		t.Errorf("auditor token: %v %v %v", id, role, ok)
	}
	if id, role, ok := st2.ByToken(s.StudentToken); !ok || id != s.ID || role != RoleStudent {
		t.Errorf("student token: %v %v %v", id, role, ok)
	}
	if _, _, ok := st2.ByToken("nope"); ok {
		t.Error("unknown token accepted")
	}
	if ids := st2.IDs(); len(ids) != 1 || ids[0] != s.ID {
		t.Errorf("IDs = %v", ids)
	}
}

func TestAcceptedFalseRoundTrip(t *testing.T) {
	f := false
	b, err := json.Marshal(Flag{ID: "f1", Accepted: &f})
	if err != nil {
		t.Fatal(err)
	}
	var back Flag
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Accepted == nil || *back.Accepted {
		t.Fatalf("false lost: %s", b)
	}
}

func TestStoreUpdateErrorLeavesSessionUnchanged(t *testing.T) {
	dir := t.TempDir()
	st, _ := OpenStore(dir)
	s := readySession(t)
	_ = st.Create(s)
	boom := errors.New("boom")
	err := st.Update(s.ID, func(x *Session) error {
		x.Fail("half-changed")
		x.Files["textkit.go"][0] = 'X'
		x.Flags = append(x.Flags, Flag{ID: "zz"})
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	st.View(s.ID, func(x *Session) {
		if x.State != StateReady || x.Files["textkit.go"][0] != 'p' || len(x.Flags) != 0 {
			t.Errorf("in-memory session changed: %s %q %v", x.State, x.Files["textkit.go"], x.Flags)
		}
	})
	st2, _ := OpenStore(dir)
	st2.View(s.ID, func(x *Session) {
		if x.State != StateReady {
			t.Errorf("disk changed: %s", x.State)
		}
	})
	if err := st.Update("missing", func(*Session) error { return nil }); err == nil {
		t.Error("update of missing session must fail")
	}
}

func TestStoreOwnsRunResultsOnCreate(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := runningSession(t)
	s.AddRun(vr(map[string]bool{"TestA": true}), false, t0)
	if err := st.Create(s); err != nil {
		t.Fatal(err)
	}
	s.Runs[0].Result.Tests[0].Status = sandbox.StatusFail
	st.View(s.ID, func(x *Session) {
		if !x.Runs[0].Result.Passed(".:TestA") {
			t.Error("caller changed stored run result")
		}
	})
}

func TestStoreRollsBackRunResults(t *testing.T) {
	for _, failWrite := range []bool{false, true} {
		name := "callback failure"
		if failWrite {
			name = "write failure"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			st, err := OpenStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			s := runningSession(t)
			s.AddRun(vr(map[string]bool{"TestA": true}), false, t0)
			if err := st.Create(s); err != nil {
				t.Fatal(err)
			}
			wantErr := errors.New("callback failed")
			if failWrite {
				// Fail in CreateTemp without changing the persisted session.
				st.dir = filepath.Join(dir, s.ID+".json")
				wantErr = nil
			}
			err = st.Update(s.ID, func(x *Session) error {
				x.Runs[0].Result.Tests[0].Status = sandbox.StatusFail
				return wantErr
			})
			if err == nil || (!failWrite && !errors.Is(err, wantErr)) {
				t.Fatalf("Update error = %v", err)
			}
			assertResult := func(x *Session) {
				if !x.Runs[0].Result.Passed(".:TestA") {
					t.Error("test result changed despite failed update")
				}
			}
			st.View(s.ID, assertResult)
			reopened, err := OpenStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			reopened.View(s.ID, assertResult)
		})
	}
}

func TestStoreFilesPrivateAndAtomic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "sessions")
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := readySession(t)
	if err := st.Create(s); err != nil {
		t.Fatal(err)
	}
	if err := st.Create(s); err == nil {
		t.Error("duplicate Create must fail")
	}
	_ = st.Update(s.ID, func(x *Session) error { x.Error = "e"; return nil })
	fi, err := os.Stat(filepath.Join(dir, s.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", fi.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("leftover files: %v", entries)
	}
}

func TestOpenStoreIgnoresJunk(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, ".tmp-123"), []byte("{broken"), 0o600)
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.IDs()) != 0 {
		t.Errorf("IDs = %v", st.IDs())
	}
}

func TestOpenStoreSkipsBadFiles(t *testing.T) {
	dir := t.TempDir()
	st, _ := OpenStore(dir)
	s := readySession(t)
	if err := st.Create(s); err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(filepath.Join(dir, s.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	// bad JSON, and a hand-copied backup whose name does not match its id
	_ = os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not json"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, s.ID+" (1).json"), good, 0o600)

	st2, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore must skip bad files: %v", err)
	}
	if ids := st2.IDs(); len(ids) != 1 || ids[0] != s.ID {
		t.Fatalf("IDs = %v, want only %s", ids, s.ID)
	}
	for _, n := range []string{"broken.json", s.ID + " (1).json"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s must stay on disk: %v", n, err)
		}
	}
}

func TestCreateRejectsUnsafeID(t *testing.T) {
	st, _ := OpenStore(t.TempDir())
	s := readySession(t)
	s.ID = "../evil"
	if err := st.Create(s); err == nil {
		t.Fatal("unsafe id accepted")
	}
}

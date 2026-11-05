package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lcme/internal/api"
	"lcme/internal/audit"
	"lcme/internal/hints"
	"lcme/internal/project"
	"lcme/internal/sandbox"
	"lcme/internal/selector"
)

func TestPreparationHasDefaultDeadline(t *testing.T) {
	f := setup(t, func(ctx context.Context, p *project.Project, progress func(selector.Progress)) (*selector.Plan, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 3*time.Minute {
			t.Error("preparation requires a deadline no later than three minutes")
		}
		return fixturePlan(ctx, p, progress)
	}, nil)
	f.ready()
}

type admissionHinter func(context.Context, []hints.Input) ([]hints.Hint, error)

func (h admissionHinter) Hints(ctx context.Context, in []hints.Input) ([]hints.Hint, error) {
	return h(ctx, in)
}

func TestPreparationTimeoutReleasesAdmission(t *testing.T) {
	for _, phase := range []string{"import", "selector", "late-success", "hints"} {
		t.Run(phase, func(t *testing.T) {
			prepare := func(ctx context.Context, p *project.Project, progress func(selector.Progress)) (*selector.Plan, error) {
				if phase == "selector" || phase == "late-success" {
					<-ctx.Done()
					if phase == "selector" {
						return nil, ctx.Err()
					}
				}
				return fixturePlan(ctx, p, progress)
			}
			f := setup(t, prepare, nil, func(d *api.Deps) {
				d.PrepareTimeout = 30 * time.Millisecond
				if phase == "import" {
					d.AllowGitImport = true
					d.Importer.Clone = func(ctx context.Context, _, _ string) error {
						<-ctx.Done()
						return ctx.Err()
					}
				}
				if phase == "hints" {
					d.Hinter = admissionHinter(func(ctx context.Context, _ []hints.Input) ([]hints.Hint, error) {
						<-ctx.Done()
						return nil, ctx.Err()
					})
				}
			})
			for range 3 { // More than the worker count proves each timeout releases admission.
				var auditor string
				if phase == "import" {
					v := f.request("POST", "/api/audits", `{"gitUrl":"https://github.com/example/demo"}`, http.StatusCreated)
					auditor = "/api" + v["auditorUrl"].(string)
				} else {
					auditor = f.create()
				}
				view := f.wait(auditor, "failed")
				if message, _ := view["error"].(string); !strings.Contains(message, "время") {
					t.Errorf("timeout must have a useful message: %q", message)
				}
			}
		})
	}
}

func TestPreparationAdmissionAndErrorRelease(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	f := setup(t, func(ctx context.Context, _ *project.Project, _ func(selector.Progress)) (*selector.Plan, error) {
		select {
		case <-release:
			return nil, selector.ErrNoTriple
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, nil)
	a := f.create()
	for range 12 {
		f.request("POST", "/api/audits", `{"example":"textkit"}`, http.StatusTooManyRequests)
	}
	assertSessionCount(t, f, 1)
	once.Do(func() { close(release) })
	f.wait(a, "failed")
	f.wait(f.create(), "failed")
}

func TestRunAndPreparationShareAdmission(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	f := setup(t, nil, func(ctx context.Context, batch sandbox.Batch) ([]sandbox.VariantResult, error) {
		select {
		case <-release:
			return pass(ctx, batch)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	a1, s1 := f.ready()
	a2, s2 := f.ready()
	a3, s3 := f.ready()
	for _, student := range []string{s1, s2, s3} {
		f.request("POST", student+"/start", "", http.StatusNoContent)
	}
	f.request("POST", s1+"/run", "", http.StatusAccepted)
	f.request("POST", s2+"/run", "", http.StatusAccepted)
	f.request("POST", s1+"/run", "", http.StatusConflict)
	f.request("POST", s3+"/run", "", http.StatusTooManyRequests)
	f.request("POST", "/api/audits", `{"example":"textkit"}`, http.StatusTooManyRequests)
	assertSessionCount(t, f, 3)
	once.Do(func() { close(release) })
	f.waitForRuns(a1, 1)
	f.waitForRuns(a2, 1)
	f.request("POST", s3+"/run", "", http.StatusAccepted)
	f.waitForRuns(a3, 1)
	f.ready()
}

func TestAdmissionReleasedAfterPersistenceFailure(t *testing.T) {
	f := setup(t, nil, nil)
	savedDir := f.storeDir + "-saved"
	if err := os.Rename(f.storeDir, savedDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.storeDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		f.request("POST", "/api/audits", `{"example":"textkit"}`, http.StatusInternalServerError)
	}
	if got := len(f.store.IDs()); got != 0 {
		t.Fatalf("failed persistence retained %d sessions", got)
	}
	if err := os.Remove(f.storeDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(savedDir, f.storeDir); err != nil {
		t.Fatal(err)
	}
	f.ready()
}

func TestAdmissionReleasedAfterRunFailure(t *testing.T) {
	f := setup(t, nil, func(context.Context, sandbox.Batch) ([]sandbox.VariantResult, error) {
		return nil, errors.New("runner unavailable")
	})
	a, student := f.ready()
	f.request("POST", student+"/start", "", http.StatusNoContent)
	for count := 1; count <= 3; count++ {
		f.request("POST", student+"/run", "", http.StatusAccepted)
		f.waitForRuns(a, count)
	}
	f.ready()
}

func TestCanceledFinalRunReleasesAdmissionAndRetries(t *testing.T) {
	entered := make(chan struct{})
	var calls atomic.Int32
	f := setup(t, nil, func(ctx context.Context, batch sandbox.Batch) ([]sandbox.VariantResult, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return pass(ctx, batch)
	})
	a, student := f.ready()
	f.request("POST", student+"/start", "", http.StatusNoContent)
	f.clock.Add(int64(11 * time.Minute))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.api.Tick(ctx)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("final run did not start")
	}
	cancel()
	end := time.Now().Add(2 * time.Second)
	for time.Now().Before(end) {
		f.api.Tick(context.Background())
		if f.request("GET", a, "", http.StatusOK)["state"] == "finished" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	f.wait(a, "finished")
	f.waitForRuns(a, 1)
	if calls.Load() != 2 {
		t.Fatalf("final attempts = %d, want one canceled and one successful", calls.Load())
	}
	f.ready()
	f.ready()
}

func TestFinalRunWaitsForAdmissionAndKeepsFinishTime(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprintf("expired=%v", expired), func(t *testing.T) {
			release := make(chan struct{})
			started := make(chan struct{}, 8)
			var once sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			f := setup(t, nil, func(ctx context.Context, batch sandbox.Batch) ([]sandbox.VariantResult, error) {
				started <- struct{}{}
				select {
				case <-release:
					return pass(ctx, batch)
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			})
			a1, s1 := f.ready()
			a2, s2 := f.ready()
			a3, s3 := f.ready()
			for _, student := range []string{s1, s2, s3} {
				f.request("POST", student+"/start", "", http.StatusNoContent)
			}
			f.request("POST", s1+"/run", "", http.StatusAccepted)
			f.request("POST", s2+"/run", "", http.StatusAccepted)
			for range 2 {
				select {
				case <-started:
				case <-time.After(2 * time.Second):
					t.Fatal("admitted run did not start")
				}
			}
			id, _, _ := f.store.ByToken(filepath.Base(s3))
			wantFinish := f.now()
			if expired {
				f.store.View(id, func(v *audit.Session) { wantFinish = v.Deadline })
				f.clock.Add(int64(11 * time.Minute))
			} else {
				f.request("POST", s3+"/finish", "", http.StatusAccepted)
			}
			for range 10 {
				f.api.Tick(context.Background())
			}
			select {
			case <-started:
				t.Fatal("final run bypassed the occupied global limit")
			case <-time.After(30 * time.Millisecond):
			}
			f.store.View(id, func(v *audit.Session) {
				if !v.FinishRequestedAt.Equal(wantFinish) || len(v.Runs) != 0 {
					t.Errorf("queued finish = %v, runs = %d", v.FinishRequestedAt, len(v.Runs))
				}
			})
			f.request("POST", s3+"/flags", `{"file":"textkit.go","line":2,"note":"late"}`, http.StatusConflict)
			once.Do(func() { close(release) })
			if expired {
				f.wait(a1, "finished")
				f.wait(a2, "finished")
			} else {
				f.waitForRuns(a1, 1)
				f.waitForRuns(a2, 1)
			}
			end := time.Now().Add(2 * time.Second)
			for time.Now().Before(end) {
				f.api.Tick(context.Background())
				if f.request("GET", a3, "", 200)["state"] == "finished" {
					break
				}
				time.Sleep(time.Millisecond)
			}
			f.wait(a3, "finished")
			f.store.View(id, func(v *audit.Session) {
				if len(v.Runs) != 1 || !v.Runs[0].Final || !v.FinishedAt.Equal(wantFinish) {
					t.Errorf("final runs = %+v, finished = %v, want %v", v.Runs, v.FinishedAt, wantFinish)
				}
			})
		})
	}
}

func TestSessionAdmissionCapIsAtomicAndPersistsAcrossRestart(t *testing.T) {
	f := setup(t, nil, nil)
	_, student := f.ready()
	for range 62 {
		if err := f.store.Create(audit.New(project.Source{Example: "textkit"}, f.now())); err != nil {
			t.Fatal(err)
		}
	}
	statuses := make(chan int, 12)
	for range 12 {
		go func() {
			res, err := f.server.Client().Post(f.server.URL+"/api/audits", "application/json", strings.NewReader(`{"example":"textkit"}`))
			if err != nil {
				statuses <- 0
				return
			}
			res.Body.Close()
			statuses <- res.StatusCode
		}()
	}
	accepted := 0
	for range 12 {
		switch status := <-statuses; status {
		case http.StatusCreated:
			accepted++
		case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		default:
			t.Errorf("unexpected create response %d", status)
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted %d new sessions with 63 retained, want 1", accepted)
	}
	assertSessionCount(t, f, 64)
	f.request("POST", student+"/start", "", http.StatusNoContent)
	f.request("POST", student+"/finish", "", http.StatusAccepted)
	f.wait(student, "finished")
	reopened, err := audit.OpenStore(f.storeDir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := api.New(api.Deps{Store: reopened, Now: f.now, Importer: &project.Importer{ExamplesDir: "../../examples"}})
	w := httptest.NewRecorder()
	restarted.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/audits", strings.NewReader(`{"example":"textkit"}`)))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("full store after restart: HTTP %d, want 503", w.Code)
	}
}

func assertSessionCount(t *testing.T, f *fixture, want int) {
	t.Helper()
	files, err := os.ReadDir(f.storeDir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			count++
		}
	}
	if got := len(f.store.IDs()); got != want || count != want {
		t.Fatalf("sessions in memory/on disk = %d/%d, want %d", got, count, want)
	}
}

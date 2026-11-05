package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"lcme/internal/api"
	"lcme/internal/audit"
	"lcme/internal/project"
	"lcme/internal/sandbox"
	"lcme/internal/selector"
)

func TestFixStudentFilesHiddenUntilStart(t *testing.T) {
	f := setup(t, nil, nil)
	a, s := f.ready()
	for _, v := range []map[string]any{f.request("GET", s, "", 200), readMessage(t, f.ws(s), "snapshot")["view"].(map[string]any)} {
		if len(v["files"].(map[string]any)) != 0 || len(v["readOnly"].([]any)) != 0 {
			t.Error("ready student sees task before timer starts")
		}
	}
	if len(f.request("GET", a, "", 200)["files"].(map[string]any)) == 0 {
		t.Fatal("auditor lost preview")
	}
	f.request("POST", s+"/start", "", 204)
	if f.request("GET", s, "", 200)["files"].(map[string]any)["textkit.go"] != code {
		t.Fatal("started task missing")
	}
}

func TestFixFinishedReviewAndUnlink(t *testing.T) {
	f := setup(t, nil, nil)
	a, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	f.request("POST", s+"/flags", `{"file":"textkit.go","line":2,"note":"boundary"}`, 201)
	f.clock.Add(int64(20 * time.Second))
	f.request("POST", s+"/run", "", 202)
	f.waitForRuns(s, 1)
	f.clock.Add(int64(10 * time.Second))
	f.request("POST", s+"/finish", "", 202)
	f.wait(a, "finished")
	f.request("POST", a+"/flags/f1", `{"mutationId":"m1","accepted":false}`, 204)
	m := f.request("GET", a, "", 200)["report"].(map[string]any)["mutations"].([]any)[0].(map[string]any)
	if m["detected"] != true || m["explained"] != false || m["fixedAfterSec"] != float64(20) {
		t.Fatalf("review changed semantics: %v", m)
	}
	f.request("POST", a+"/flags/f1", `{"mutationId":"","accepted":false}`, 204)
	m = f.request("GET", a, "", 200)["report"].(map[string]any)["mutations"].([]any)[0].(map[string]any)
	if m["detected"] != false || m["fixedAfterSec"] != float64(20) {
		t.Fatalf("unlink failed: %v", m)
	}
}

func TestFixRejectSourceBeforePersistence(t *testing.T) {
	f := setup(t, nil, nil, func(d *api.Deps) { d.AllowGitImport = true })
	for _, body := range []string{`{"gitUrl":"https://invalid.example/repo"}`, `{"example":"missing"}`} {
		f.request("POST", "/api/audits", body, 400)
	}
	assertSessionCount(t, f, 0)
}

func TestFixPreparationDiagnosticIsSafeAndUseful(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&selector.BaselineError{Kind: "tests", FailingTests: []string{".:TestBoundary"}}, "TestBoundary"},
		{&selector.BaselineError{Kind: "build"}, "не компилируется"},
		{selector.ErrInfrastructure, "Среда подготовки"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			f := setup(t, func(context.Context, *project.Project, func(selector.Progress)) (*selector.Plan, error) {
				return nil, tc.err
			}, nil)
			v := f.wait(f.create(), "failed")
			if msg := v["error"].(string); !strings.Contains(msg, tc.want) {
				t.Fatalf("diagnostic %q does not explain %s", msg, tc.want)
			}
		})
	}
}

func TestFixReserveRunCapacity(t *testing.T) {
	var block atomic.Bool
	release := make(chan struct{})
	defer close(release)
	f := setup(t, func(ctx context.Context, p *project.Project, progress func(selector.Progress)) (*selector.Plan, error) {
		if block.Load() {
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return fixturePlan(ctx, p, progress)
	}, nil)
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	block.Store(true)
	a := f.create()
	f.request("POST", "/api/audits", `{"example":"textkit"}`, 429)
	f.request("POST", s+"/run", "", 202)
	f.waitForRuns(s, 1)
	release <- struct{}{}
	f.wait(a, "ready")
}

func TestFixCreationRateRecovers(t *testing.T) {
	f := setup(t, nil, nil)
	// A small burst is useful for a demo, but it must not fill all 64 slots.
	for range 6 {
		f.ready()
	}
	f.request("POST", "/api/audits", `{"example":"textkit"}`, 429)
	assertSessionCount(t, f, 6)
	f.clock.Add(int64(time.Minute))
	f.ready()
}

func TestFixRetentionAndRestartRecovery(t *testing.T) {
	f := setup(t, nil, nil)
	ids := map[audit.State]string{}
	for _, state := range []audit.State{audit.StateFailed, audit.StateReady, audit.StateFinished, audit.StateRunning, audit.StatePreparing} {
		v := audit.New(project.Source{Example: "textkit"}, f.now().Add(-48*time.Hour))
		v.State = state
		v.FinishedAt = v.CreatedAt
		if state == audit.StateRunning {
			v.StartedAt = f.now()
			v.Deadline = f.now().Add(10 * time.Minute)
		}
		if err := f.store.Create(v); err != nil {
			t.Fatal(err)
		}
		ids[state] = v.ID
	}
	restarted := api.New(api.Deps{Store: f.store, Now: f.now, Runner: runner(pass), WorkDir: f.workDir})
	f.store.View(ids[audit.StatePreparing], func(v *audit.Session) {
		if v.State != audit.StateFailed || v.Error == "" {
			t.Error("preparing not recovered")
		}
	})
	restarted.Tick(context.Background())
	for _, state := range []audit.State{audit.StateFailed, audit.StateReady, audit.StateFinished} {
		f.store.View(ids[state], func(*audit.Session) { t.Errorf("expired %s retained", state) })
	}
	exists := false
	f.store.View(ids[audit.StateRunning], func(*audit.Session) { exists = true })
	if !exists {
		t.Fatal("active audit removed")
	}
	reloaded, err := audit.OpenStore(f.storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []audit.State{audit.StateFailed, audit.StateReady, audit.StateFinished} {
		reloaded.View(ids[state], func(*audit.Session) { t.Errorf("expired %s persists on disk", state) })
	}
}

func TestFixInfrastructureRetryKeepsFrozenSnapshot(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	defer close(release)
	f := setup(t, nil, func(ctx context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("docker secret /host/path")
		}
		if got, err := os.ReadFile(filepath.Join(b.Dir, b.IDs[0], "textkit.go")); err != nil || string(got) != code {
			t.Errorf("retry did not use frozen code: %q, %v", got, err)
		}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return pass(ctx, b)
	})
	a, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	f.clock.Add(int64(35 * time.Second))
	f.request("POST", s+"/finish", "", 202)
	v := f.wait(a, "finished")
	report := v["report"].(map[string]any)
	if msg, _ := report["verificationError"].(string); msg == "" || strings.Contains(msg, "secret") {
		t.Fatalf("missing safe verification error: %v", report)
	}
	result := v["runs"].([]any)[0].(map[string]any)["result"].(map[string]any)
	if msg, _ := f.request("GET", s, "", 200)["report"].(map[string]any)["verificationError"].(string); msg == "" {
		t.Error("student cannot distinguish unavailable assessment")
	}
	if result["buildError"] != "" && result["buildError"] != nil {
		t.Fatal("infrastructure reported as compile error")
	}
	for range 3 {
		f.api.Tick(context.Background())
	}
	if calls.Load() != 1 {
		t.Fatal("automatic retry loop")
	}
	f.request("POST", s+"/finish", "", 409)
	f.clock.Add(int64(time.Hour))
	f.request("POST", a+"/finish", "", 202)
	if got := f.request("GET", a, "", 200); got["running"] != true || got["state"] != "finished" {
		t.Fatalf("retry state: %v", got)
	}
	f.request("POST", s+"/flags", `{"file":"textkit.go","line":2,"note":"late"}`, 409)
	conn := f.ws(s)
	readMessage(t, conn, "snapshot")
	writeMessage(t, conn, map[string]any{"type": "file", "path": "textkit.go", "content": "changed after finish", "revision": 1})
	readMessage(t, conn, "error")
	release <- struct{}{}
	f.waitForRuns(a, 2)
	report = f.request("GET", a, "", 200)["report"].(map[string]any)
	if report["restored"] != true || report["durationSec"] != float64(35) || report["verificationError"] != nil {
		t.Fatalf("retry report: %v", report)
	}
}

func TestFixRunInfrastructureErrors(t *testing.T) {
	for _, kind := range []string{"runner", "timeout", "missing", "result"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t, nil, func(context.Context, sandbox.Batch) ([]sandbox.VariantResult, error) {
				switch kind {
				case "runner":
					return nil, errors.New("/host/private secret")
				case "timeout":
					return nil, context.DeadlineExceeded
				case "result":
					return []sandbox.VariantResult{{ID: "v", InfrastructureError: "/host/private secret"}}, nil
				}
				return nil, nil
			})
			a, s := f.ready()
			f.request("POST", s+"/start", "", 204)
			f.request("POST", s+"/finish", "", 202)
			v := f.wait(a, "finished")
			data, _ := json.Marshal(v)
			if strings.Contains(string(data), "/host/private") {
				t.Fatal("infrastructure diagnostic leaked")
			}
			r := v["report"].(map[string]any)
			if r["verificationError"] == nil {
				t.Fatal("missing explicit error")
			}
		})
	}
}

func TestFixServerHeartbeat(t *testing.T) {
	f := setup(t, nil, nil)
	_, s := f.ready()
	conn := f.ws(s)
	readMessage(t, conn, "snapshot")
	ctx, cancel := context.WithTimeout(context.Background(), 17*time.Second)
	defer cancel()
	var msg map[string]any
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		t.Fatal(err)
	}
	if msg["type"] != "ping" {
		t.Fatalf("idle heartbeat=%v", msg)
	}
}

func TestFixWebSocketConnectionLimit(t *testing.T) {
	f := setup(t, nil, nil)
	_, s := f.ready()
	for range 8 {
		readMessage(t, f.ws(s), "snapshot")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, res, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(f.server.URL, "http")+"/ws/"+filepath.Base(s), nil)
	if err == nil {
		conn.CloseNow()
		t.Fatal("unbounded websocket peers")
	}
	if res == nil || res.StatusCode != 429 {
		t.Fatalf("want 429, got %v", res)
	}
}

func TestFixSecurityAndStaticCaching(t *testing.T) {
	f := setup(t, nil, nil, func(d *api.Deps) {
		d.Static = fstest.MapFS{"index.html": {Data: []byte("index")}, "assets/index-abc12345.js": {Data: []byte("js")}}
	})
	for _, path := range []string{"/a/token", "/index.html", "/assets/index-abc12345.js"} {
		rec := httptest.NewRecorder()
		f.api.Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Header().Get("X-Frame-Options") != "DENY" {
			t.Error("missing frame restriction")
		}
		want := "no-store"
		if strings.HasPrefix(path, "/assets/") {
			want = "public, max-age=31536000, immutable"
		}
		if rec.Header().Get("Cache-Control") != want {
			t.Errorf("cache %s: %s", path, rec.Header().Get("Cache-Control"))
		}
	}
}

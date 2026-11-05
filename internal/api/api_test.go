package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"lcme/internal/api"
	"lcme/internal/audit"
	"lcme/internal/mutate"
	"lcme/internal/project"
	"lcme/internal/sandbox"
	"lcme/internal/selector"
)

const code = "package textkit\nfunc Value() int { return 3 }\n"
const testFile = "package textkit\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n"

type runner func(context.Context, sandbox.Batch) ([]sandbox.VariantResult, error)

func (r runner) Run(ctx context.Context, batch sandbox.Batch) ([]sandbox.VariantResult, error) {
	return r(ctx, batch)
}

func (r runner) GoVersion(context.Context) (string, error) { return "go1.24", nil }

func pass(_ context.Context, batch sandbox.Batch) ([]sandbox.VariantResult, error) {
	return []sandbox.VariantResult{{ID: batch.IDs[0], Tests: []sandbox.TestResult{{Pkg: ".", Name: "TestValue", Status: sandbox.StatusPass}}}}, nil
}

func fixturePlan(_ context.Context, p *project.Project, progress func(selector.Progress)) (*selector.Plan, error) {
	progress(selector.Progress{Stage: "done", Done: 1, Total: 1})
	files := map[string][]byte{"textkit.go": []byte(code), "textkit_test.go": p.Files["textkit_test.go"], "go.mod": p.Files["go.mod"]}
	return &selector.Plan{
		Mutations: []mutate.Mutation{
			{ID: "m1", File: "textkit.go", Line: 2, Orig: "secretOriginal", Repl: "3"},
			{ID: "m2", File: "textkit.go", Line: 2},
			{ID: "m3", File: "textkit.go", Line: 2},
		},
		Mutated: files, KillingTests: [][]string{{".:TestValue"}, {".:TestValue"}, {".:TestValue"}}, BaselineTests: []string{".:TestValue"},
	}, nil
}

type fixture struct {
	t        *testing.T
	api      *api.Server
	server   *httptest.Server
	store    *audit.Store
	storeDir string
	workDir  string
	clock    atomic.Int64
}

func setup(t *testing.T, prepare api.PrepareFunc, run runner, configure ...func(*api.Deps)) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{t: t, storeDir: filepath.Join(root, "state"), workDir: filepath.Join(root, "work")}
	f.clock.Store(time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC).UnixNano())
	examples := filepath.Join(root, "examples")
	example := filepath.Join(examples, "textkit")
	if err := os.MkdirAll(example, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"go.mod": "module textkit\ngo 1.24\n", "textkit.go": strings.Replace(code, "return 3", "return 2", 1), "textkit_test.go": testFile} {
		if err := os.WriteFile(filepath.Join(example, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	f.store, err = audit.OpenStore(f.storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if prepare == nil {
		prepare = fixturePlan
	}
	if run == nil {
		run = pass
	}
	deps := api.Deps{Store: f.store, Importer: &project.Importer{ExamplesDir: examples, WorkDir: f.workDir, MaxGo: "1.24"}, Prepare: prepare, Runner: run, WorkDir: f.workDir, Now: f.now}
	for _, configure := range configure {
		configure(&deps)
	}
	f.api = api.New(deps)
	f.server = httptest.NewServer(f.api.Handler())
	t.Cleanup(f.server.Close)
	return f
}

func (f *fixture) now() time.Time { return time.Unix(0, f.clock.Load()).UTC() }

func (f *fixture) request(method, path, body string, want int) map[string]any {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	res, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != want {
		f.t.Fatalf("%s %s status %d, want %d", method, path, res.StatusCode, want)
	}
	view := map[string]any{}
	if res.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(res.Body).Decode(&view); err != nil {
			f.t.Fatal(err)
		}
	}
	return view
}

func (f *fixture) create() string {
	f.t.Helper()
	v := f.request("POST", "/api/audits", `{"example":"textkit"}`, 201)
	p, _ := v["auditorUrl"].(string)
	if !strings.HasPrefix(p, "/a/") {
		f.t.Fatalf("bad auditor URL %q", p)
	}
	return "/api" + p
}

func (f *fixture) wait(path, state string) map[string]any {
	f.t.Helper()
	end := time.Now().Add(2 * time.Second)
	for {
		v := f.request("GET", path, "", 200)
		if v["state"] == state {
			return v
		}
		if time.Now().After(end) {
			f.t.Fatalf("state %v, want %s; view: %v", v["state"], state, v)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (f *fixture) waitForRuns(path string, count int) {
	f.t.Helper()
	end := time.Now().Add(2 * time.Second)
	for {
		v := f.request("GET", path, "", 200)
		if len(v["runs"].([]any)) == count {
			return
		}
		if time.Now().After(end) {
			f.t.Fatalf("runs %v, want %d", v["runs"], count)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (f *fixture) ready() (auditor, student string) {
	f.t.Helper()
	auditor = f.create()
	v := f.wait(auditor, "ready")
	p, _ := v["studentUrl"].(string)
	if !strings.HasPrefix(p, "/s/") {
		f.t.Fatalf("bad student URL %q", p)
	}
	return auditor, "/api" + p
}

func (f *fixture) ws(path string) *websocket.Conn {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	token := filepath.Base(path)
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(f.server.URL, "http")+"/ws/"+token, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	conn.SetReadLimit(2 << 20)
	f.t.Cleanup(func() { _ = conn.CloseNow() })
	return conn
}

func readMessage(t *testing.T, conn *websocket.Conn, kind string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		v := map[string]any{}
		if err := wsjson.Read(ctx, conn, &v); err != nil {
			t.Fatal(err)
		}
		if v["type"] == kind {
			return v
		}
	}
}

func writeMessage(t *testing.T, conn *websocket.Conn, v map[string]any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, v); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAuditExampleBecomesReady(t *testing.T) {
	setup(t, nil, nil).ready()
}

func TestStudentViewHasNoAnswers(t *testing.T) {
	f := setup(t, nil, nil)
	a, s := f.ready()
	assertNoAnswers := func(v map[string]any) {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{`"mutations"`, `"hints"`, `"killingTests"`, `"mutated"`, `"orig"`, `"repl"`, `"mutationId"`, "secretOriginal"} {
			if bytes.Contains(raw, []byte(secret)) {
				t.Errorf("student data contains %s", secret)
			}
		}
	}
	v := f.request("GET", s, "", 200)
	assertNoAnswers(v)
	f.request("POST", s+"/start", "", 204)
	v = f.request("GET", s, "", 200)
	if got := v["files"].(map[string]any)["textkit.go"]; got != code {
		t.Fatalf("student file = %q", got)
	}
	f.request("POST", s+"/flags", `{"file":"textkit.go","line":2,"note":"граница"}`, 201)
	f.request("POST", a+"/flags/f1", `{"mutationId":"m1","accepted":true}`, 204)
	f.request("POST", s+"/finish", "", 202)
	v = f.wait(s, "finished")
	assertNoAnswers(v)
	assertNoAnswers(readMessage(t, f.ws(s), "snapshot")["view"].(map[string]any))
	if v["report"].(map[string]any)["restored"] != true {
		t.Fatal("no student summary")
	}
	report := f.wait(a, "finished")["report"].(map[string]any)
	mutation := report["mutations"].([]any)[0].(map[string]any)
	if mutation["orig"] != "secretOriginal" || mutation["explained"] != true {
		t.Fatalf("incomplete auditor report: %v", report)
	}
}

func TestStudentCannotEditTests(t *testing.T) {
	f := setup(t, nil, nil)
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	c := f.ws(s)
	readMessage(t, c, "snapshot")
	writeMessage(t, c, map[string]any{"type": "file", "path": "textkit_test.go", "content": "forged", "revision": 2})
	err := readMessage(t, c, "error")
	if err["path"] != "textkit_test.go" || err["revision"] != float64(2) {
		t.Fatalf("file error cannot be matched: %v", err)
	}
	if v := f.request("GET", s, "", 200)["files"].(map[string]any)["textkit_test.go"]; v != testFile {
		t.Fatalf("changed test file: %v", v)
	}
}

func TestRunUsesCanonicalTests(t *testing.T) {
	seen := make(chan []byte, 1)
	f := setup(t, nil, func(ctx context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
		got, err := os.ReadFile(filepath.Join(b.Dir, b.IDs[0], "textkit_test.go"))
		seen <- got
		if err != nil {
			return nil, err
		}
		return pass(ctx, b)
	})
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	f.request("POST", s+"/run", "", 202)
	select {
	case got := <-seen:
		if string(got) != testFile {
			t.Fatalf("run had noncanonical tests: %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner not invoked")
	}
	f.waitForRuns(s, 1)
}

func TestRunConcurrent409(t *testing.T) {
	entered, release := make(chan struct{}, 1), make(chan struct{})
	defer close(release)
	f := setup(t, nil, func(ctx context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
		entered <- struct{}{}
		select {
		case <-release:
			return pass(ctx, b)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	f.request("POST", s+"/run", "", 202)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("runner not invoked")
	}
	f.request("POST", s+"/run", "", 409)
	release <- struct{}{}
	f.waitForRuns(s, 1)
}

func TestReconnectSnapshotSameDeadline(t *testing.T) {
	f := setup(t, nil, nil)
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	first := f.ws(s)
	deadline := readMessage(t, first, "snapshot")["view"].(map[string]any)["deadline"]
	_ = first.CloseNow()
	f.clock.Add(int64(time.Second))
	next := readMessage(t, f.ws(s), "snapshot")["view"].(map[string]any)
	if deadline != "2026-11-05T09:10:00Z" || deadline != next["deadline"] {
		t.Fatalf("deadlines %v and %v", deadline, next["deadline"])
	}
}

func TestAuditorGetsLiveFile(t *testing.T) {
	f := setup(t, nil, nil)
	a, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	ac, sc := f.ws(a), f.ws(s)
	readMessage(t, ac, "snapshot")
	readMessage(t, sc, "snapshot")
	// More than the default WS limit, with JSON escaping close to its worst case.
	content := strings.Repeat("\x01", 256<<10)
	writeMessage(t, sc, map[string]any{"type": "file", "path": "textkit.go", "content": content, "revision": 1})
	ack := readMessage(t, sc, "fileAck")
	if ack["path"] != "textkit.go" || ack["content"] != content || ack["revision"] != float64(1) {
		t.Fatal("file ack does not match input")
	}
	msg := readMessage(t, ac, "file")
	if msg["path"] != "textkit.go" || msg["content"] != content {
		t.Fatal("incorrect live file")
	}
	if got := f.request("GET", s, "", 200)["files"].(map[string]any)["textkit.go"]; got != content {
		t.Fatal("acknowledged edit was not stored")
	}
}

func checkFinal(t *testing.T, v map[string]any) {
	t.Helper()
	runs := v["runs"].([]any)
	if len(runs) == 0 || runs[len(runs)-1].(map[string]any)["final"] != true {
		t.Fatalf("missing final run: %v", runs)
	}
}

func TestTimerFinalizes(t *testing.T) {
	f := setup(t, nil, nil)
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	f.clock.Add(int64(10*time.Minute + time.Second))
	f.api.Tick(context.Background())
	checkFinal(t, f.wait(s, "finished"))
}

func TestExpiredOnLoadFinalized(t *testing.T) {
	f := setup(t, nil, nil)
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	f.clock.Add(int64(10*time.Minute + time.Second))
	reloaded, err := audit.OpenStore(f.storeDir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := api.New(api.Deps{Store: reloaded, Runner: runner(pass), WorkDir: f.workDir, Now: f.now})
	server := httptest.NewServer(restarted.Handler())
	defer server.Close()
	f.server = server
	restarted.Tick(context.Background())
	checkFinal(t, f.wait(s, "finished"))
}

func TestPrepareFailureShownToAuditor(t *testing.T) {
	f := setup(t, func(context.Context, *project.Project, func(selector.Progress)) (*selector.Plan, error) {
		return nil, selector.ErrNoTriple
	}, nil)
	v := f.wait(f.create(), "failed")
	if text, _ := v["error"].(string); !strings.Contains(text, "мутац") {
		t.Fatalf("expected a Russian explanation, got %q", text)
	}
}

func TestDeadlineQueuesFinalRunBehindActiveRun(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls atomic.Int32
	f := setup(t, nil, func(ctx context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
		if calls.Add(1) == 1 {
			entered <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return pass(ctx, b)
	})
	defer close(release)
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	f.request("POST", s+"/run", "", 202)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("runner not invoked")
	}
	f.clock.Add(int64(10 * time.Minute))
	f.api.Tick(context.Background())
	v := f.request("GET", s, "", 200)
	if v["finishing"] != true || v["running"] != true || calls.Load() != 1 {
		t.Fatalf("finish was not queued behind the active run: %v", v)
	}
	release <- struct{}{}
	v = f.wait(s, "finished")
	checkFinal(t, v)
	runs := v["runs"].([]any)
	if len(runs) != 2 || runs[0].(map[string]any)["final"] != false {
		t.Fatalf("bad run order: %v", runs)
	}
}

func TestFinishRejectsNewChanges(t *testing.T) {
	f := setup(t, nil, nil)
	a, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	c := f.ws(s)
	readMessage(t, c, "snapshot")
	f.request("POST", a+"/finish", "", 202)
	writeMessage(t, c, map[string]any{"type": "file", "path": "textkit.go", "content": "too late"})
	readMessage(t, c, "error")
	f.request("POST", s+"/flags", `{"file":"textkit.go","line":1,"note":"too late"}`, 409)
	v := f.wait(s, "finished")
	if v["files"].(map[string]any)["textkit.go"] != code {
		t.Fatal("late change was applied")
	}
}

func TestWrongRoleGetsForbiddenAndLocalGitIsRejected(t *testing.T) {
	f := setup(t, nil, nil)
	a, s := f.ready()
	f.request("GET", "/api/s/"+filepath.Base(a), "", 403)
	f.request("GET", "/api/a/"+filepath.Base(s), "", 403)
	f.request("POST", "/api/audits", `{"gitUrl":"https://example.com/project.git"}`, 403)
}

func TestWebSocketEventAndAuditorCamelCase(t *testing.T) {
	f := setup(t, nil, nil)
	a, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	ac, sc := f.ws(a), f.ws(s)
	first := readMessage(t, ac, "snapshot")["view"].(map[string]any)
	keys := []string{"stage", "done", "total"}
	for key := range first["progress"].(map[string]any) {
		if !slices.Contains(keys, key) {
			t.Fatalf("non-camel-case progress key %q", key)
		}
	}
	readMessage(t, sc, "snapshot")
	writeMessage(t, sc, map[string]any{"type": "event", "kind": "paste", "size": 7})
	next := readMessage(t, ac, "snapshot")["view"].(map[string]any)
	events := next["events"].([]any)
	if len(events) != 1 || events[0].(map[string]any)["size"] != float64(7) {
		t.Fatalf("missing paste event: %v", events)
	}
}

func TestManualFinishResumesFromStore(t *testing.T) {
	f := setup(t, nil, nil)
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	f.clock.Add(int64(35 * time.Second))
	id, _, ok := f.store.ByToken(filepath.Base(s))
	if !ok {
		t.Fatal("session not stored")
	}
	if err := f.store.Update(id, func(v *audit.Session) error {
		v.FinishRequestedAt = f.now()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(int64(20 * time.Second))
	reloaded, err := audit.OpenStore(f.storeDir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := api.New(api.Deps{Store: reloaded, Runner: runner(pass), WorkDir: f.workDir, Now: f.now})
	server := httptest.NewServer(restarted.Handler())
	defer server.Close()
	f.server = server
	f.request("POST", s+"/run", "", 409)
	restarted.Tick(context.Background())
	v := f.wait(s, "finished")
	checkFinal(t, v)
	if got := v["report"].(map[string]any)["durationSec"]; got != float64(35) {
		t.Fatalf("duration = %v, want persisted freeze time at 35 seconds", got)
	}
}

func TestLocalRejectsRunOfPersistedImportedSource(t *testing.T) {
	var calls atomic.Int32
	f := setup(t, nil, func(ctx context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
		calls.Add(1)
		return pass(ctx, b)
	})
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	id, _, ok := f.store.ByToken(filepath.Base(s))
	if !ok {
		t.Fatal("session not stored")
	}
	if err := f.store.Update(id, func(v *audit.Session) error {
		v.Source = project.Source{GitURL: "https://example.com/project.git"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.request("POST", s+"/run", "", 403)
	f.request("POST", s+"/finish", "", 202)
	v := f.wait(s, "finished")
	checkFinal(t, v)
	if calls.Load() != 0 || v["report"].(map[string]any)["restored"] != false {
		t.Fatalf("untrusted project ran in local mode: %v", v)
	}
}

func TestBodyLimits(t *testing.T) {
	f := setup(t, nil, nil)
	f.request("POST", "/api/audits", `{"example":"`+strings.Repeat("x", 20<<10)+`"}`, 413)
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	f.request("POST", s+"/run", strings.Repeat("x", 20<<10), 413)

	c := f.ws(s)
	readMessage(t, c, "snapshot")
	writeMessage(t, c, map[string]any{"type": "file", "path": "textkit.go", "content": strings.Repeat("x", 1+audit.MaxFileSize)})
	readMessage(t, c, "error")
	if got := f.request("GET", s, "", 200)["files"].(map[string]any)["textkit.go"]; got != code {
		t.Fatal("oversized edit changed source")
	}
}

func TestCanonicalFilesCapturedBeforeConcurrentEdits(t *testing.T) {
	entered := make(chan string, 2)
	release := make(chan struct{})
	var calls atomic.Int32
	f := setup(t, nil, func(ctx context.Context, b sandbox.Batch) ([]sandbox.VariantResult, error) {
		if calls.Add(1) == 1 {
			entered <- "waiting"
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		got, err := os.ReadFile(filepath.Join(b.Dir, b.IDs[0], "textkit.go"))
		if err != nil {
			return nil, err
		}
		entered <- string(got)
		return pass(ctx, b)
	})
	defer close(release)
	readCapture := func(want string) {
		t.Helper()
		select {
		case got := <-entered:
			if got != want {
				t.Fatalf("captured %q, want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("runner did not capture the source")
		}
	}
	_, s := f.ready()
	f.request("POST", s+"/start", "", 204)
	c := f.ws(s)
	readMessage(t, c, "snapshot")
	f.request("POST", s+"/run", "", 202)
	readCapture("waiting")
	edited := strings.Replace(code, "return 3", "return 2", 1)
	writeMessage(t, c, map[string]any{"type": "file", "path": "textkit.go", "content": edited})
	readMessage(t, c, "fileAck")
	f.request("POST", s+"/finish", "", 202)
	release <- struct{}{}
	readCapture(code)
	readCapture(edited)
	checkFinal(t, f.wait(s, "finished"))
}

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadConfigDefaultsAndLocalGuard(t *testing.T) {
	for _, k := range []string{"LCME_ADDR", "LCME_RUNNER", "LCME_PUBLIC_URL", "LCME_DATA_DIR", "LCME_WORK_DIR", "LCME_RUNNER_BIN", "LCME_RUNNER_IMAGE"} {
		t.Setenv(k, "")
	}
	c, err := loadConfig()
	if err != nil || c.mode != "docker" || c.addr != ":8080" {
		t.Fatalf("%+v %v", c, err)
	}
	t.Setenv("LCME_RUNNER", "local")
	c, err = loadConfig()
	if err != nil || c.addr != "127.0.0.1:8080" {
		t.Fatalf("%+v %v", c, err)
	}
	t.Setenv("LCME_ADDR", ":8080")
	if _, err = loadConfig(); err == nil {
		t.Fatal("local exposed on all interfaces")
	}
	t.Setenv("LCME_ADDR", "127.0.0.1:8080")
	t.Setenv("LCME_RUNNER", "typo")
	if _, err = loadConfig(); err == nil {
		t.Fatal("unknown runner accepted")
	}
}

func TestLocalHandlerRejectsNonLoopbackHost(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, host := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080", "attacker.example:8080", "127.0.0.1.attacker.example"} {
		req := httptest.NewRequest("GET", "http://localhost/healthz", nil)
		req.Host = host
		w := httptest.NewRecorder()
		localOnly(next).ServeHTTP(w, req)
		want := http.StatusForbidden
		if host == "127.0.0.1:8080" || host == "localhost:8080" || host == "[::1]:8080" {
			want = http.StatusNoContent
		}
		if w.Code != want {
			t.Fatalf("%s: %d want %d", host, w.Code, want)
		}
	}
}

func TestSessionSelectionUsesUnpredictableSeed(t *testing.T) {
	seen := map[int64]bool{}
	for i := 0; i < 8; i++ {
		opt, err := sessionSelectionOptions()
		if err != nil {
			t.Fatal(err)
		}
		if opt.Seed == 0 || seen[opt.Seed] {
			t.Fatalf("non-random session seed: %d", opt.Seed)
		}
		if opt.BaselineRuns != 3 || opt.MaxCandidates != 64 {
			t.Fatalf("lost production limits: %+v", opt)
		}
		seen[opt.Seed] = true
	}
}

package hints

import (
	"context"
	"encoding/json"
	"lcme/internal/mutate"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func sample() []Input {
	return []Input{{M: mutate.Mutation{ID: "m1", Kind: mutate.Boundary, Orig: "<", Repl: "<=", Line: 2}, Snippet: "if x < 2 {}"}}
}
func TestTemplateCoversAllKinds(t *testing.T) {
	for _, k := range []mutate.Kind{mutate.Boundary, mutate.Negate, mutate.Logical, mutate.Arith, mutate.IncDec, mutate.Constant} {
		in := sample()
		in[0].M.Kind = k
		got, err := Template().Hints(context.Background(), in)
		if err != nil || len(got) != 1 || got[0].Explanation == "" || got[0].Question == "" || got[0].Source != "template" {
			t.Fatalf("%s: %+v %v", k, got, err)
		}
	}
}
func TestOpenRouterRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("wrong request")
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "m" || len(body.Messages) != 2 {
			t.Errorf("body %+v %v", body, err)
		}
		if len(body.Messages) == 2 {
			var inputs []Input
			if err := json.Unmarshal([]byte(body.Messages[1].Content), &inputs); err != nil || len(inputs) != 1 || inputs[0].Snippet != "if x < 2 {}" {
				t.Errorf("wrong snippet: %+v %v", inputs, err)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": "[{\"mutationId\":\"m1\",\"explanation\":\"e\",\"question\":\"q\"}]"}}}})
	}))
	defer srv.Close()
	got, err := OpenRouter("k", "m", srv.URL, srv.Client()).Hints(context.Background(), sample())
	if err != nil || len(got) != 1 || got[0].Source != "llm" || got[0].Explanation != "e" {
		t.Fatalf("%+v %v", got, err)
	}
}
func TestOpenRouterBadJSONFallsBack(t *testing.T) {
	for _, content := range []string{"не JSON", "[]", `[{"mutationId":"wrong","explanation":"e","question":"q"}]`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": content}}}})
		}))
		got, err := Fallback(OpenRouter("k", "m", srv.URL, srv.Client()), Template()).Hints(context.Background(), sample())
		srv.Close()
		if err != nil || len(got) != 1 || got[0].Source != "template" {
			t.Fatalf("%q: %+v %v", content, got, err)
		}
	}
}
func TestOpenRouterTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	defer close(release)
	started := time.Now()
	got, err := Fallback(OpenRouter("k", "m", srv.URL, srv.Client()), Template()).Hints(ctx, sample())
	if err != nil || len(got) != 1 || got[0].Source != "template" || time.Since(started) > time.Second {
		t.Fatalf("%+v %v", got, err)
	}
}
func TestSnippetBounds(t *testing.T) {
	src := []byte("0\n1\n2\n3\n4\n5\n6\n7\n8\n")
	if got := Snippet(src, 5); got != "1\n2\n3\n4\n5\n6\n7" {
		t.Fatalf("%q", got)
	}
	if got := Snippet(src, 0); got != "" {
		t.Fatal(got)
	}
}

package hints

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

const validHintJSON = `[{"mutationId":"m1","explanation":"Граничное значение проходит другую ветку.","question":"Как проверить границу диапазона?"}]`

func TestOpenRouterOversizedInputUsesTemplateWithoutRequest(t *testing.T) {
	for _, mode := range []string{"snippet", "metadata", "escaped"} {
		t.Run(mode, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				completion(w, validHintJSON, "stop")
			}))
			defer srv.Close()
			in := []Input{{}}
			in[0].M.ID = "m1"
			switch mode {
			case "snippet":
				in[0].Snippet = strings.Repeat("x", 65537)
			case "metadata":
				in[0].M.Func = strings.Repeat("x", 65537)
			case "escaped":
				in[0].Snippet = strings.Repeat("\x00", 12000)
			}
			got, err := Fallback(OpenRouter("k", "m", srv.URL, srv.Client()), Template()).Hints(context.Background(), in)
			if err != nil || len(got) != 1 || got[0].Source != "template" || requests != 0 {
				t.Fatalf("oversized prompt must fall back without network: hints=%d requests=%d err=%v", len(got), requests, err)
			}
		})
	}
}

func completion(w io.Writer, content, reason string) {
	json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
		"finish_reason": reason,
		"message":       map[string]string{"role": "assistant", "content": content},
	}}})
}

func TestOpenRouterStructuredRequest(t *testing.T) {
	in := sample()
	in = append(in, in[0])
	in[1].M.ID = "m2"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			MaxTokens int `json:"max_tokens"`
			Reasoning struct {
				Effort  string `json:"effort"`
				Exclude bool   `json:"exclude"`
			} `json:"reasoning"`
			Provider struct {
				RequireParameters bool `json:"require_parameters"`
			} `json:"provider"`
			ResponseFormat struct {
				Type       string `json:"type"`
				JSONSchema struct {
					Name   string         `json:"name"`
					Strict bool           `json:"strict"`
					Schema map[string]any `json:"schema"`
				} `json:"json_schema"`
			} `json:"response_format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.MaxTokens <= 0 || request.MaxTokens > 4096 || request.Reasoning.Effort != "minimal" || !request.Reasoning.Exclude {
			t.Errorf("missing bounded generation: tokens=%d reasoning=%+v", request.MaxTokens, request.Reasoning)
		}
		if !request.Provider.RequireParameters || request.ResponseFormat.Type != "json_schema" || !request.ResponseFormat.JSONSchema.Strict || request.ResponseFormat.JSONSchema.Name == "" {
			t.Error("structured output must be enforced by provider routing")
		}
		want := map[string]any{
			"type": "object", "required": []any{"hints"}, "additionalProperties": false,
			"properties": map[string]any{"hints": map[string]any{
				"type": "array", "minItems": float64(2), "maxItems": float64(2),
				"items": map[string]any{
					"type": "object", "required": []any{"mutationId", "explanation", "question"}, "additionalProperties": false,
					"properties": map[string]any{
						"mutationId":  map[string]any{"type": "string", "enum": []any{"m1", "m2"}},
						"explanation": map[string]any{"type": "string"},
						"question":    map[string]any{"type": "string"},
					},
				},
			}},
		}
		if !reflect.DeepEqual(request.ResponseFormat.JSONSchema.Schema, want) {
			t.Errorf("schema does not describe the complete hint batch: %+v", request.ResponseFormat.JSONSchema.Schema)
		}
		// The provider may return hints in a different order; IDs must bind them.
		completion(w, `{"hints":[{"mutationId":"m2","explanation":"второе","question":"вопрос 2?"},{"mutationId":"m1","explanation":"первое","question":"вопрос 1?"}]}`, "stop")
	}))
	defer srv.Close()
	got, err := OpenRouter("k", "m", srv.URL, srv.Client()).Hints(context.Background(), in)
	if err != nil || len(got) != 2 || got[0].MutationID != "m1" || got[0].Explanation != "первое" || got[1].MutationID != "m2" || got[1].Explanation != "второе" || got[0].Source != "llm" || got[1].Source != "llm" {
		t.Fatalf("ordered hints: %+v, %v", got, err)
	}
}

func TestOpenRouterRejectsIncompleteOrInvalidContent(t *testing.T) {
	for _, tc := range []struct {
		name, content, reason string
	}{
		{"length despite valid JSON", validHintJSON, "length"},
		{"filtered", validHintJSON, "content_filter"},
		{"generation error", validHintJSON, "error"},
		{"tool call", validHintJSON, "tool_calls"},
		{"missing completion reason", validHintJSON, ""},
		{"whitespace explanation", `[{"mutationId":"m1","explanation":" \n\t","question":"вопрос?"}]`, "stop"},
		{"whitespace question", `[{"mutationId":"m1","explanation":"пояснение","question":" \n\t"}]`, "stop"},
		{"long explanation", `[{"mutationId":"m1","explanation":"` + strings.Repeat("я", 481) + `","question":"вопрос?"}]`, "stop"},
		{"long question", `[{"mutationId":"m1","explanation":"пояснение","question":"` + strings.Repeat("я", 241) + `"}]`, "stop"},
		{"unexpected answer field", `[{"mutationId":"m1","explanation":"пояснение","question":"вопрос?","answer":"заменить оператор"}]`, "stop"},
		{"wrong wrapper", `{"other":` + validHintJSON + `}`, "stop"},
		{"extra wrapper field", `{"hints":` + validHintJSON + `,"extra":"ответ"}`, "stop"},
		{"incomplete wrapper", `{"hints":` + validHintJSON + `,`, "stop"},
		{"trailing JSON", validHintJSON + ` {"extra":true}`, "stop"},
		{"truncated JSON", `[{"mutationId":"m1"`, "stop"},
		{"missing question", `[{"mutationId":"m1","explanation":"пояснение"}]`, "stop"},
		{"unknown mutation", `[{"mutationId":"unknown","explanation":"пояснение","question":"вопрос?"}]`, "stop"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				completion(w, tc.content, tc.reason)
			}))
			defer srv.Close()
			got, err := Fallback(OpenRouter("k", "m", srv.URL, srv.Client()), Template()).Hints(context.Background(), sample())
			if err != nil || len(got) != 1 || got[0].Source != "template" || got[0].MutationID != "m1" {
				t.Fatalf("invalid completion must fall back: %+v, %v", got, err)
			}
		})
	}
}

func TestOpenRouterRejectsIncompleteOrDuplicateBatch(t *testing.T) {
	in := sample()
	in = append(in, in[0])
	in[1].M.ID = "m2"
	for _, content := range []string{
		validHintJSON,
		`[{"mutationId":"m1","explanation":"e","question":"q"},{"mutationId":"m1","explanation":"e","question":"q"}]`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			completion(w, content, "stop")
		}))
		got, err := Fallback(OpenRouter("k", "m", srv.URL, srv.Client()), Template()).Hints(context.Background(), in)
		srv.Close()
		if err != nil || len(got) != 2 || got[0].Source != "template" || got[1].Source != "template" || got[0].MutationID != "m1" || got[1].MutationID != "m2" {
			t.Fatalf("invalid batch must fall back as a whole: %+v, %v", got, err)
		}
	}
}

func TestOpenRouterInvalidEnvelopeFallsBack(t *testing.T) {
	var successful strings.Builder
	completion(&successful, validHintJSON, "stop")
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"unsupported schema", `{"error":{"message":"unsupported response_format"}}`, http.StatusBadRequest},
		{"rate limit", `{"error":{"message":"rate limit"}}`, http.StatusTooManyRequests},
		{"invalid envelope", `{`, http.StatusOK},
		{"no choices", `{"choices":[]}`, http.StatusOK},
		{"null completion reason", strings.Replace(successful.String(), `"stop"`, `null`, 1), http.StatusOK},
		{"content is not text", `{"choices":[{"finish_reason":"stop","message":{"content":[]}}]}`, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			got, err := Fallback(OpenRouter("k", "m", srv.URL, srv.Client()), Template()).Hints(context.Background(), sample())
			if err != nil || len(got) != 1 || got[0].Source != "template" || got[0].MutationID != "m1" {
				t.Fatalf("failed request must fall back: %+v, %v", got, err)
			}
		})
	}
}

func TestOpenRouterTextLimitsCountRunes(t *testing.T) {
	explanation, question := strings.Repeat("я", 480), strings.Repeat("я", 240)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		completion(w, `[{"mutationId":"m1","explanation":" `+explanation+` ","question":" `+question+` "}]`, "stop")
	}))
	defer srv.Close()
	got, err := OpenRouter("k", "m", srv.URL, srv.Client()).Hints(context.Background(), sample())
	if err != nil || len(got) != 1 || got[0].Explanation != explanation || got[0].Question != question || got[0].Source != "llm" {
		t.Fatalf("valid Russian text at character limits must be accepted and trimmed: %+v, %v", got, err)
	}
}

type hintTransport func(*http.Request) (*http.Response, error)

func (f hintTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOpenRouterBoundsCustomClientDeadline(t *testing.T) {
	c := &http.Client{Transport: hintTransport(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 20*time.Second {
			t.Errorf("request must have a deadline within 20s, got %v (%v)", deadline, ok)
		}
		var body strings.Builder
		completion(&body, validHintJSON, "stop")
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body.String())), Header: make(http.Header)}, nil
	})}
	got, err := OpenRouter("k", "m", "http://openrouter.invalid", c).Hints(context.Background(), sample())
	if err != nil || len(got) != 1 || got[0].Source != "llm" {
		t.Fatalf("%+v, %v", got, err)
	}
}

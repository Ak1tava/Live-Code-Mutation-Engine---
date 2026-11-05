// Package hints holds the hint type attached to an audit session.
package hints

import (
	"context"
	"fmt"
	"lcme/internal/mutate"
	"strings"
)

// Hint is an explanation and a guiding question for one mutation.
type Hint struct {
	MutationID  string `json:"mutationId"`
	Explanation string `json:"explanation"`
	Question    string `json:"question"`
	Source      string `json:"source"`
}

type Input struct {
	M       mutate.Mutation `json:"mutation"`
	Snippet string          `json:"snippet"`
}

type Hinter interface {
	Hints(context.Context, []Input) ([]Hint, error)
}
type hinterFunc func(context.Context, []Input) ([]Hint, error)

func (f hinterFunc) Hints(ctx context.Context, in []Input) ([]Hint, error) { return f(ctx, in) }

// Template needs no network, so it also works after an LLM timeout.
func Template() Hinter {
	return hinterFunc(func(_ context.Context, in []Input) ([]Hint, error) {
		out := make([]Hint, 0, len(in))
		questions := map[mutate.Kind]string{
			mutate.Boundary: "Что должно происходить на границе диапазона?",
			mutate.Negate:   "Когда это условие должно быть истинным?",
			mutate.Logical:  "Должны выполняться оба условия или достаточно одного?",
			mutate.Arith:    "Как операция влияет на вычисляемое значение?",
			mutate.IncDec:   "Как меняется счётчик и когда закончится цикл?",
			mutate.Constant: "Почему здесь выбрано именно это значение?",
		}
		for _, v := range in {
			q := questions[v.M.Kind]
			if q == "" {
				q = "Какое поведение ожидается в этой строке?"
			}
			out = append(out, Hint{MutationID: v.M.ID, Explanation: fmt.Sprintf("Замена %q на %q меняет логику выражения. Проверьте случаи, которые различают эти варианты.", v.M.Orig, v.M.Repl), Question: q, Source: "template"})
		}
		return out, nil
	})
}
func Fallback(primary, secondary Hinter) Hinter {
	return hinterFunc(func(ctx context.Context, in []Input) ([]Hint, error) {
		if primary != nil {
			if out, err := primary.Hints(ctx, in); err == nil {
				return out, nil
			}
		}
		return secondary.Hints(ctx, in)
	})
}

// Snippet returns at most three lines on each side of a physical source line.
func Snippet(src []byte, line int) string {
	lines := strings.Split(string(src), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	lo, hi := max(0, line-4), min(len(lines), line+3)
	return strings.Join(lines[lo:hi], "\n")
}

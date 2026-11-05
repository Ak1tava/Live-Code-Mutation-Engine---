package mutate

import (
	"errors"
	"sort"
	"strings"
	"testing"
)

const src = "package a\n\nfunc F(n int) bool {\n\tfor i := 0; i < n; i++ {\n\t\tif i == 3 && n > 1 {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n"

func TestGenerateFindsOperators(t *testing.T) {
	files := map[string][]byte{"a.go": []byte(src)}
	ms, err := Generate(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 8 {
		t.Fatalf("len = %d, want 8: %+v", len(ms), ms)
	}
	m0 := ms[0]
	if m0.ID != "c1" || m0.Kind != "constant" || m0.Orig != "0" || m0.Repl != "1" || m0.Line != 4 || m0.Func != "F" {
		t.Errorf("ms[0] = %+v", m0)
	}
	var kinds []string
	for i, m := range ms {
		if got := string(files[m.File][m.Start:m.End]); got != m.Orig {
			t.Errorf("%s: slice %q != orig %q", m.ID, got, m.Orig)
		}
		if want := "c" + string(rune('1'+i)); m.ID != want {
			t.Errorf("ID = %s, want %s", m.ID, want)
		}
		kinds = append(kinds, string(m.Kind)+":"+m.Orig+">"+m.Repl)
	}
	sort.Strings(kinds)
	want := []string{
		"boundary:<><=", "boundary:>>>=", "constant:0>1", "constant:1>2", "constant:3>4",
		"incdec:++>--", "logical:&&>||", "negate:==>!=",
	}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Errorf("kinds = %v, want %v", kinds, want)
	}
}

func TestApplyChangesExactlyOneLine(t *testing.T) {
	files := map[string][]byte{"a.go": []byte(src)}
	ms, _ := Generate(files)
	out, err := Apply(files, ms[1:2])
	if err != nil {
		t.Fatal(err)
	}
	a, b := strings.Split(src, "\n"), strings.Split(string(out["a.go"]), "\n")
	if len(a) != len(b) {
		t.Fatalf("line count changed")
	}
	diff := 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
	}
	if diff != 1 {
		t.Errorf("diff lines = %d, want 1", diff)
	}
	if string(files["a.go"]) != src {
		t.Error("input mutated")
	}
}

func TestApplyRejectsOverlap(t *testing.T) {
	files := map[string][]byte{"a.go": []byte(src)}
	ms, _ := Generate(files)
	_, err := Apply(files, []Mutation{ms[0], ms[0]})
	if !errors.Is(err, ErrOverlap) {
		t.Errorf("err = %v, want ErrOverlap", err)
	}
}

func TestGenerateSkipsBigConstants(t *testing.T) {
	s := "package a\n\nfunc G() int {\n\treturn 100 + 99\n}\n"
	ms, err := Generate(map[string][]byte{"a.go": []byte(s)})
	if err != nil {
		t.Fatal(err)
	}
	var consts []Mutation
	for _, m := range ms {
		if m.Kind == "constant" {
			consts = append(consts, m)
		}
	}
	if len(consts) != 1 || consts[0].Orig != "99" || consts[0].Repl != "100" {
		t.Errorf("consts = %+v", consts)
	}
}

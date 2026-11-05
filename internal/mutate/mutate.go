// Package mutate finds mutation candidates in Go sources via go/ast and
// applies them by byte-offset replacement.
package mutate

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
)

// Kind is the category of a mutation.
type Kind string

const (
	Boundary Kind = "boundary"
	Negate   Kind = "negate"
	Logical  Kind = "logical"
	Arith    Kind = "arith"
	IncDec   Kind = "incdec"
	Constant Kind = "constant"
)

// Mutation is a single candidate replacement of bytes [Start, End) in File.
type Mutation struct {
	ID    string `json:"id"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Func  string `json:"func"`
	Kind  Kind   `json:"kind"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Orig  string `json:"orig"`
	Repl  string `json:"repl"`
}

// ErrOverlap is returned by Apply when two mutations touch overlapping ranges.
var ErrOverlap = errors.New("mutations overlap")

const maxConstant = 100

type opSwap struct {
	repl string
	kind Kind
}

var binarySwaps = map[token.Token]opSwap{
	token.LSS: {"<=", Boundary}, token.LEQ: {"<", Boundary},
	token.GTR: {">=", Boundary}, token.GEQ: {">", Boundary},
	token.EQL: {"!=", Negate}, token.NEQ: {"==", Negate},
	token.LAND: {"||", Logical}, token.LOR: {"&&", Logical},
	token.ADD: {"-", Arith}, token.SUB: {"+", Arith},
}

// Generate returns all mutation candidates for the given non-test .go files,
// ordered by (file, offset) and numbered c1..cN.
func Generate(files map[string][]byte) ([]Mutation, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var all []Mutation
	for _, name := range names {
		ms, err := generateFile(name, files[name])
		if err != nil {
			return nil, err
		}
		all = append(all, ms...)
	}
	for i := range all {
		all[i].ID = "c" + strconv.Itoa(i+1)
	}
	return all, nil
}

func generateFile(name string, src []byte) ([]Mutation, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	var ms []Mutation
	add := func(fn string, pos token.Pos, length int, kind Kind, repl string) {
		// The editor and report refer to the physical file, even with //line directives.
		p := fset.PositionFor(pos, false)
		ms = append(ms, Mutation{
			File: name, Line: p.Line, Func: fn, Kind: kind,
			Start: p.Offset, End: p.Offset + length,
			Orig: string(src[p.Offset : p.Offset+length]), Repl: repl,
		})
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		fn := fd.Name.Name
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BinaryExpr:
				if sw, ok := binarySwaps[x.Op]; ok {
					add(fn, x.OpPos, len(x.Op.String()), sw.kind, sw.repl)
				}
			case *ast.IncDecStmt:
				repl := "--"
				if x.Tok == token.DEC {
					repl = "++"
				}
				add(fn, x.TokPos, 2, IncDec, repl)
			case *ast.BasicLit:
				if x.Kind != token.INT {
					break
				}
				v, err := strconv.ParseInt(x.Value, 0, 64)
				if err != nil || v < 0 || v >= maxConstant {
					break
				}
				add(fn, x.Pos(), len(x.Value), Constant, strconv.FormatInt(v+1, 10))
			}
			return true
		})
	}
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].Start < ms[j].Start })
	return ms, nil
}

// Apply returns a copy of files with ms applied. Mutations within one file
// must not overlap; otherwise ErrOverlap is returned.
func Apply(files map[string][]byte, ms []Mutation) (map[string][]byte, error) {
	out := make(map[string][]byte, len(files))
	for name, b := range files {
		out[name] = append([]byte(nil), b...)
	}
	byFile := map[string][]Mutation{}
	for _, m := range ms {
		src, ok := files[m.File]
		if !ok {
			return nil, fmt.Errorf("mutation %s: unknown file %q", m.ID, m.File)
		}
		if m.Start < 0 || m.Start > m.End || m.End > len(src) {
			return nil, fmt.Errorf("mutation %s: range [%d,%d) out of bounds", m.ID, m.Start, m.End)
		}
		byFile[m.File] = append(byFile[m.File], m)
	}
	for name, list := range byFile {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Start < list[j].Start })
		for i := 1; i < len(list); i++ {
			if list[i].Start < list[i-1].End || list[i].Start == list[i-1].Start {
				return nil, fmt.Errorf("%w: %s and %s in %s", ErrOverlap, list[i-1].ID, list[i].ID, name)
			}
		}
		b := out[name]
		for i := len(list) - 1; i >= 0; i-- {
			m := list[i]
			b = append(b[:m.Start:m.Start], append([]byte(m.Repl), b[m.End:]...)...)
		}
		out[name] = b
	}
	return out, nil
}

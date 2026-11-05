package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"lcme/internal/sandbox"
)

// discoverAndInstrument reads the build-selected test files without executing
// project init or TestMain. The Go tool still validates test signatures when
// compiling. An overlay adds a completion callback without changing /work.
func discoverAndInstrument(dir, pkg, bin string) ([]string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-json", "./"+pkg)
	cmd.Dir = dir
	sandbox.SetProcessGroup(cmd)
	out := &boundedBuffer{limit: 1 << 20}
	errout := &tail{n: sandbox.OutputCap}
	cmd.Stdout, cmd.Stderr = out, errout
	err := cmd.Run()
	sandbox.KillGroup(cmd)
	if err != nil {
		return nil, "", fmt.Errorf("discover tests: %w: %s", err, errout.bytes())
	}
	if out.overflow {
		return nil, "", fmt.Errorf("test package metadata exceeds 1 MiB")
	}
	var info struct {
		Dir                       string
		TestGoFiles, XTestGoFiles []string
	}
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		return nil, "", fmt.Errorf("test package metadata: %w", err)
	}
	work, err := os.MkdirTemp(filepath.Dir(bin), "instrument-")
	if err != nil {
		return nil, "", err
	}
	// One helper per internal/external test package. A fresh identifier avoids
	// collisions with imported code; it is not a secret or an authentication key.
	helper := "lcmeCompletion_" + strings.TrimPrefix(filepath.Base(work), "instrument-")
	replace := map[string]string{}
	packages := map[string]bool{}
	seen := map[string]bool{}
	var names []string
	for i, name := range append(info.TestGoFiles, info.XTestGoFiles...) {
		path := filepath.Join(info.Dir, name)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, "", err
		}
		changed := false
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isTestFunction(fn) {
				continue
			}
			if seen[fn.Name.Name] {
				return nil, "", fmt.Errorf("ambiguous test name %s in package %s", fn.Name.Name, pkg)
			}
			seen[fn.Name.Name] = true
			names = append(names, fn.Name.Name)
			param := fn.Type.Params.List[0]
			if len(param.Names) == 0 || param.Names[0].Name == "_" {
				param.Names = []*ast.Ident{ast.NewIdent(helper + "Arg")}
			}
			call := &ast.ExprStmt{X: &ast.CallExpr{
				Fun: ast.NewIdent(helper), Args: []ast.Expr{ast.NewIdent(param.Names[0].Name)},
			}}
			fn.Body.List = append([]ast.Stmt{call}, fn.Body.List...)
			changed = true
		}
		if !changed {
			continue
		}
		packages[file.Name.Name] = true
		var src bytes.Buffer
		if err := format.Node(&src, fset, file); err != nil {
			return nil, "", err
		}
		backing := filepath.Join(work, strconv.Itoa(i)+".go")
		if err := os.WriteFile(backing, src.Bytes(), 0600); err != nil {
			return nil, "", err
		}
		replace[path] = backing
	}
	for pkgName := range packages {
		backing := filepath.Join(work, pkgName+"_test.go")
		src := fmt.Sprintf(completionHelper, pkgName, helper)
		if err := os.WriteFile(backing, []byte(src), 0600); err != nil {
			return nil, "", err
		}
		replace[filepath.Join(info.Dir, helper+"_"+pkgName+"_test.go")] = backing
	}
	data, err := json.Marshal(struct{ Replace map[string]string }{replace})
	if err != nil {
		return nil, "", err
	}
	overlay := filepath.Join(work, "overlay.json")
	if err := os.WriteFile(overlay, data, 0600); err != nil {
		return nil, "", err
	}
	sort.Strings(names)
	return names, overlay, nil
}

// Match cmd/go's Test naming and *T signature rules, including TestMain(*T),
// dot/aliased imports, unnamed parameters, and Unicode test names. Invalid
// signatures are left for go test -c to reject, never treated as passing tests.
func isTestFunction(fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	if fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(name, "Test") {
		return false
	}
	if len(name) > 4 {
		r, _ := utf8.DecodeRuneInString(name[4:])
		if unicode.IsLower(r) {
			return false
		}
	}
	if fn.Type.Results != nil && len(fn.Type.Results.List) != 0 || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	param := fn.Type.Params.List[0]
	ptr, ok := param.Type.(*ast.StarExpr)
	if !ok || len(param.Names) > 1 {
		return false
	}
	switch typ := ptr.X.(type) {
	case *ast.Ident:
		return typ.Name == "T"
	case *ast.SelectorExpr:
		return typ.Sel.Name == "T"
	}
	return false
}

// Register before the test body, so this cleanup runs after its cleanups and
// parallel subtests. Skips are not passes. The dedicated FD prevents stdout
// text (including forged test2json events) from being mistaken for evidence.
//
// This is NOT an attestation against arbitrary hostile code: it shares the
// test process/address space/UID, and such code can write FD 3 or tamper with
// testing state. Docker isolates the host, not the test harness from its own
// process. Defending that stronger threat requires a different trust boundary.
const completionHelper = `package %[1]s
import (
 lcmeJSON "encoding/json"
 lcmeOS "os"
 lcmeTesting "testing"
)
var %[2]sFile = lcmeOS.NewFile(3, "lcme-completion")
func %[2]s(t *lcmeTesting.T) {
 t.Cleanup(func() {
  f := %[2]sFile
  if f == nil { return }
  _ = lcmeJSON.NewEncoder(f).Encode(struct {
   Name string
   Failed, Skipped bool
  }{t.Name(), t.Failed(), t.Skipped()})
 })
}
`

type boundedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if n > b.limit-b.Len() {
		p = p[:b.limit-b.Len()]
		b.overflow = true
	}
	b.Buffer.Write(p)
	return n, nil
}

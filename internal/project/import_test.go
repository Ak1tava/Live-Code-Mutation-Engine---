package project

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const examplesDir = "../../examples"

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture builds a repo layout; files overrides the default pair of a.go and a_test.go.
func fixture(t *testing.T, gomod string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if gomod == "" {
		gomod = "module example.com/fx\n\ngo 1.22\n"
	}
	writeFile(t, dir, "go.mod", gomod)
	if files == nil {
		files = map[string]string{
			"a.go":      "package fx\n",
			"a_test.go": "package fx\n",
		}
	}
	for k, v := range files {
		writeFile(t, dir, k, v)
	}
	return dir
}

// copyCloner is a fake Cloner that copies a local fixture, symlinks included.
func copyCloner(src string) Cloner {
	return func(ctx context.Context, url, dst string) error {
		return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(src, p)
			target := filepath.Join(dst, rel)
			switch {
			case d.IsDir():
				return os.MkdirAll(target, 0o755)
			case d.Type()&fs.ModeSymlink != 0:
				l, err := os.Readlink(p)
				if err != nil {
					return err
				}
				return os.Symlink(l, target)
			default:
				b, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				return os.WriteFile(target, b, 0o644)
			}
		})
	}
}

func newImporter(t *testing.T, src string) *Importer {
	t.Helper()
	im := &Importer{
		ExamplesDir: examplesDir,
		WorkDir:     t.TempDir(),
		Limits:      DefaultLimits,
		MaxGo:       "1.26",
	}
	if src != "" {
		im.Clone = copyCloner(src)
	}
	return im
}

const goodURL = "https://github.com/example/repository.git"

// Validation must run before any cloner (and therefore any outbound request).
func TestImportRejectsUnsafeEndpoint(t *testing.T) {
	for _, raw := range []string{
		"https://127.0.0.1/a/b", "https://[::1]/a/b", "https://169.254.169.254/a/b",
		"https://internal.example/a/b", "https://github.com.evil.test/a/b",
		"https://user:secret@github.com/a/b", "https://github.com:8443/a/b",
		"https://github.com./a/b", "https://github.com/a/b?url=https://127.0.0.1",
		"https://github.com/a/b#fragment", "https://github.com/a/%2e%2e", "https://github.com/a/b/extra",
	} {
		t.Run(raw, func(t *testing.T) {
			im := newImporter(t, "")
			called := false
			im.Clone = func(context.Context, string, string) error { called = true; return ErrClone }
			_, err := im.Import(context.Background(), Source{GitURL: raw})
			if !errors.Is(err, ErrBadURL) || called {
				t.Fatalf("unsafe URL reached cloner=%v; err=%v", called, err)
			}
		})
	}
}

func TestImportExample(t *testing.T) {
	im := newImporter(t, "")
	p, err := im.Import(context.Background(), Source{Example: "textkit"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Module != "example.com/textkit" {
		t.Errorf("module = %q", p.Module)
	}
	if p.GoDirective != "1.22" {
		t.Errorf("go = %q", p.GoDirective)
	}
	if len(p.SourceFiles) != 1 || p.SourceFiles[0] != "textkit.go" {
		t.Errorf("SourceFiles = %v", p.SourceFiles)
	}
	if len(p.TestFiles) != 1 || p.TestFiles[0] != "textkit_test.go" {
		t.Errorf("TestFiles = %v", p.TestFiles)
	}
	if _, ok := p.GoFiles()["textkit.go"]; !ok || len(p.GoFiles()) != 1 {
		t.Errorf("GoFiles = %v", p.GoFiles())
	}
}

func TestImportRejectsUnknownExample(t *testing.T) {
	im := newImporter(t, "")
	for _, name := range []string{"../../etc", "..", ".", "a/b", `a\b`, "nope", "textkit/../textkit", "textkit/go.mod", "textkit.."} {
		_, err := im.Import(context.Background(), Source{Example: name})
		if !errors.Is(err, ErrUnknownExample) {
			t.Errorf("%q: err = %v", name, err)
		}
	}
	// A regular file in ExamplesDir is not an example either.
	dir := t.TempDir()
	writeFile(t, dir, "plain", "x")
	im.ExamplesDir = dir
	if _, err := im.Import(context.Background(), Source{Example: "plain"}); !errors.Is(err, ErrUnknownExample) {
		t.Errorf("file example: err = %v", err)
	}
}

func TestImportRejectsBadURL(t *testing.T) {
	im := newImporter(t, fixture(t, "", nil))
	for _, u := range []string{"http://x/y", "file:///etc", "--upload-pack=x", "git@github.com:a/b", "", "https://", "-https://x/y"} {
		_, err := im.Import(context.Background(), Source{GitURL: u})
		if !errors.Is(err, ErrBadURL) {
			t.Errorf("%q: err = %v", u, err)
		}
	}
}

func TestImportGitOK(t *testing.T) {
	im := newImporter(t, fixture(t, "", nil))
	p, err := im.Import(context.Background(), Source{GitURL: goodURL})
	if err != nil {
		t.Fatal(err)
	}
	if p.Module != "example.com/fx" || len(p.SourceFiles) != 1 {
		t.Errorf("unexpected project %+v", p)
	}
	entries, _ := os.ReadDir(im.WorkDir)
	if len(entries) != 0 {
		t.Errorf("work dir not cleaned: %v", entries)
	}
}

func TestImportCloneError(t *testing.T) {
	im := newImporter(t, "")
	im.Clone = func(ctx context.Context, url, dst string) error { return ErrClone }
	_, err := im.Import(context.Background(), Source{GitURL: goodURL})
	if !errors.Is(err, ErrClone) {
		t.Errorf("err = %v", err)
	}
}

func TestImportStripsGitAndSymlinks(t *testing.T) {
	dir := fixture(t, "", nil)
	writeFile(t, dir, ".git/config", "[core]\n")
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "secret")); err != nil {
		t.Fatal(err)
	}
	im := newImporter(t, dir)
	p, err := im.Import(context.Background(), Source{GitURL: goodURL})
	if err != nil {
		t.Fatal(err)
	}
	for k := range p.Files {
		if k == "secret" || strings.HasPrefix(k, ".git/") {
			t.Errorf("unexpected key %q", k)
		}
	}
	if _, ok := p.Files["go.mod"]; !ok {
		t.Error("go.mod missing from Files")
	}
}

func TestImportRejectsRequire(t *testing.T) {
	for _, gm := range []string{
		"module m\n\ngo 1.22\n\nrequire github.com/x/y v1.0.0\n",
		"module m\n\ngo 1.22\n\nrequire (\n\tgithub.com/x/y v1.0.0\n)\n",
	} {
		im := newImporter(t, fixture(t, gm, nil))
		_, err := im.Import(context.Background(), Source{GitURL: goodURL})
		if !errors.Is(err, ErrExternalDeps) {
			t.Errorf("err = %v", err)
		}
	}
}

func TestImportGoDirectiveWithTab(t *testing.T) {
	im := newImporter(t, fixture(t, "module\tm\n\ngo\t1.99\n", nil))
	if _, err := im.Import(context.Background(), Source{GitURL: goodURL}); !errors.Is(err, ErrGoTooNew) {
		t.Errorf("go<tab>1.99: err = %v", err)
	}
	im = newImporter(t, fixture(t, "module m\n\ngo 1.22\n\nrequire\tgithub.com/x/y v1.0.0\n", nil))
	if _, err := im.Import(context.Background(), Source{GitURL: goodURL}); !errors.Is(err, ErrExternalDeps) {
		t.Errorf("require<tab>: err = %v", err)
	}
	im = newImporter(t, fixture(t, "module m\n\ngo 1.22\n\nrequire(\n\tgithub.com/x/y v1.0.0\n)\n", nil))
	if _, err := im.Import(context.Background(), Source{GitURL: goodURL}); !errors.Is(err, ErrExternalDeps) {
		t.Errorf("require(: err = %v", err)
	}
}

func TestParseGoModNotFooled(t *testing.T) {
	m, v, req := parseGoMod("module m\ngo 1.22\nrequirefoo bar\n")
	if m != "m" || v != "1.22" || req {
		t.Errorf("got %q %q %v", m, v, req)
	}
}

func TestImportRejectsNoGoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", "package a\n")
	im := newImporter(t, dir)
	_, err := im.Import(context.Background(), Source{GitURL: goodURL})
	if !errors.Is(err, ErrNoGoMod) {
		t.Errorf("err = %v", err)
	}
}

func TestImportRejectsNoTests(t *testing.T) {
	im := newImporter(t, fixture(t, "", map[string]string{"a.go": "package fx\n"}))
	_, err := im.Import(context.Background(), Source{GitURL: goodURL})
	if !errors.Is(err, ErrNoTests) {
		t.Errorf("err = %v", err)
	}
}

func TestImportRejectsNewGo(t *testing.T) {
	im := newImporter(t, fixture(t, "module m\n\ngo 1.99\n", nil))
	_, err := im.Import(context.Background(), Source{GitURL: goodURL})
	if !errors.Is(err, ErrGoTooNew) {
		t.Errorf("err = %v", err)
	}
	// Major/minor and patch versions must all fit the available toolchain.
	for _, v := range []string{"1.9", "1.26.1"} {
		im = newImporter(t, fixture(t, "module m\n\ngo "+v+"\n", nil))
		im.MaxGo = "1.26.1"
		if _, err := im.Import(context.Background(), Source{GitURL: goodURL}); err != nil {
			t.Errorf("go %s should be accepted: %v", v, err)
		}
	}
}

func TestImportLimits(t *testing.T) {
	dir := fixture(t, "", map[string]string{
		"a.go": "package fx\n", "a_test.go": "package fx\n", "b.go": "package fx\n",
	}) // go.mod + 3 files = 4
	im := newImporter(t, dir)
	im.Limits = Limits{MaxFiles: 3}
	if _, err := im.Import(context.Background(), Source{GitURL: goodURL}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("files: err = %v", err)
	}
	im.Limits = Limits{MaxBytes: 10}
	if _, err := im.Import(context.Background(), Source{GitURL: goodURL}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("bytes: err = %v", err)
	}
}

func TestSourceFilesSkipsNonCompiled(t *testing.T) {
	dir := fixture(t, "", map[string]string{
		"a.go":                    "package fx\n",
		"a_test.go":               "package fx\n",
		"sub/b.go":                "package sub\n",
		"sub/b_test.go":           "package sub\n",
		"testdata/broken.go":      "not go",
		"testdata/broken_test.go": "not go",
		"vendor/v/v.go":           "not go",
		"sub/vendor/v.go":         "not go",
		".hidden/h.go":            "not go",
		"_skip/s_test.go":         "not go",
		"_u.go":                   "not go",
		".h.go":                   "not go",
	})
	im := newImporter(t, dir)
	p, err := im.Import(context.Background(), Source{GitURL: goodURL})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(p.SourceFiles, ","); got != "a.go,sub/b.go" {
		t.Errorf("SourceFiles = %s", got)
	}
	if got := strings.Join(p.TestFiles, ","); got != "a_test.go,sub/b_test.go" {
		t.Errorf("TestFiles = %s", got)
	}
	for _, k := range []string{"testdata/broken.go", "vendor/v/v.go", ".hidden/h.go", "_u.go"} {
		if _, ok := p.Files[k]; !ok {
			t.Errorf("%s should stay in Files", k)
		}
	}
}

func TestGitCloneFailsFast(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	start := time.Now()
	err := GitClone(context.Background(), "https://127.0.0.1:1/x.git", filepath.Join(t.TempDir(), "d"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrClone) {
		t.Errorf("err = %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("took %v", d)
	}
}

func TestUserMessages(t *testing.T) {
	latin := regexp.MustCompile(`[A-Za-z]{4,}`)
	technical := regexp.MustCompile(`https|github|require|test`) // допустимые технические термины
	errs := []error{ErrBadURL, ErrClone, ErrNoGoMod, ErrExternalDeps, ErrNoTests, ErrTooLarge, ErrGoTooNew, ErrUnknownExample}
	for _, e := range errs {
		m := UserMessage(e)
		if m == "" {
			t.Errorf("%v: empty message", e)
		}
		if strings.Contains(m, e.Error()) || latin.MatchString(technical.ReplaceAllString(m, "")) {
			t.Errorf("%v: message %q contains English", e, m)
		}
	}
	if UserMessage(errors.New("boom")) == "" {
		t.Error("fallback message empty")
	}
}
